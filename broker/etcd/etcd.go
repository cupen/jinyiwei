package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// "github.com/coreos/etcd/clientv3"

	"github.com/cupen/xdisco/broker"
	"github.com/cupen/xdisco/eventhandler"
	"github.com/cupen/xdisco/health"
	"github.com/cupen/xdisco/logs"
	"github.com/cupen/xdisco/server"
	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/time/rate"
)

var (
	log  = logs.Logger("info")
	log2 = log.Sugar()
)

type Etcd struct {
	baseKey   string
	cfg       clientv3.Config
	client    *clientv3.Client
	cacheKeys map[string]struct{}
	stateCh   chan server.State
	ttl       time.Duration
}

func New(baseKey string, ttl time.Duration) (*Etcd, error) {
	cfg := clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 3 * time.Second,
	}
	if baseKey == "" {
		return nil, fmt.Errorf("empty baseKey")
	}
	return NewWithConfig(baseKey, ttl, cfg)
}

func NewWithConfig(baseKey string, ttl time.Duration, cfg clientv3.Config) (*Etcd, error) {
	if baseKey == "" {
		return nil, fmt.Errorf("empty baseKey")
	}
	if ttl <= time.Second {
		return nil, fmt.Errorf("invalid ttl(%v). it must bigger than 1 second", ttl)
	}
	c, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, fmt.Errorf("failed to connect etcd. cfg: %+v", cfg)
	}
	return &Etcd{
		baseKey:   baseKey,
		cfg:       cfg,
		client:    c,
		cacheKeys: map[string]struct{}{},
		ttl:       ttl,
	}, nil
}

func (e *Etcd) initWatch() {
	if e.stateCh != nil {
		close(e.stateCh)
	}
	e.stateCh = make(chan server.State)
}

func (e *Etcd) Watch(ctx context.Context, kind string, h eventhandler.Handler, checker health.Checker) error {
	if !h.IsValid() {
		return fmt.Errorf("invalid eventhandler")
	}
	now := time.Now()
	fullKey := e.buildKeyOfList(kind)
	log2.Infof("[etcd] watch<%s> starting: %s", kind, fullKey)
	watchCh := e.client.Watch(ctx, fullKey, clientv3.WithPrefix())

	pingTS := time.Now()
	// servers, err := e.fetchServersAlived(fullKey, checker)
	servers, err := e.fetchServers(fullKey)
	if err != nil {
		return err
	}
	log2.Infof("[etcd] %d servers<%s> found", len(servers), kind)
	h.OnInit(servers)
	for _, s := range servers {
		e.cacheKeys[s.GetKey()] = struct{}{}
	}
	pingCost := time.Since(pingTS)
	e.initWatch()
	go e.startWatch(ctx, watchCh, h)
	log2.Infof("[etcd] watch<%s> started: %s cost:%v, ping:%v", kind, fullKey, time.Since(now), pingCost)
	return nil
}

func (e *Etcd) startWatch(ctx context.Context, watchCh clientv3.WatchChan, h eventhandler.Handler) {
	limit := rate.NewLimiter(rate.Every(time.Second), 100)
	for {
		select {
		case resp := <-watchCh:
			if err := resp.Err(); err != nil {
				log2.Error("[etcd] invalid watch response. err:%v", err)
				limit.Wait(ctx)
				continue
			}
			if err := e.handleWatchResponse(h, &resp); err != nil {
				log2.Error("[etcd] invalid watch response. err:%v", err)
				limit.Wait(ctx)
				continue
			}
		case <-ctx.Done():
			return
		}
	}
}

// Ping ...
func (e *Etcd) Ping() error {
	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
	defer cancel()
	var pingKey = "_ping_"
	var pingValue = "_pong_"
	c := e.client
	if _, err := c.Put(ctx, pingKey, pingValue); err != nil {
		return err
	}
	if resp, err := c.Get(ctx, pingKey); err != nil {
		return err
	} else {
		if len(resp.Kvs) <= 0 {
			return fmt.Errorf("ping failed: no value")
		}
		rs := string(resp.Kvs[0].Value)
		if rs != pingValue {
			return fmt.Errorf("ping failed: invalid pong value: %s", rs)
		}
	}
	return nil
}

func (e *Etcd) handleWatchResponse(h eventhandler.Handler, resp *clientv3.WatchResponse) error {
	if !h.IsValid() {
		panic(fmt.Errorf("invalid handler"))
	}
	if err := resp.Err(); err != nil {
		return err
	}
	if len(resp.Events) <= 0 {
		return fmt.Errorf("empty etcd events")
	}
	for _, ev := range resp.Events {
		key := string(ev.Kv.Key)
		switch ev.Type {
		case clientv3.EventTypePut:
			s, err := server.NewServerFromEtcd(key, ev.Kv.Value)
			if err != nil {
				return fmt.Errorf("invalid event data: parsing failed. key=%s err:%w. ", key, err)
			}
			if _, exists := e.cacheKeys[key]; !exists {
				h.OnAdd(key, s)
			} else {
				h.OnUpdate(key, s)
			}
			e.cacheKeys[key] = struct{}{}
		case clientv3.EventTypeDelete:
			delete(e.cacheKeys, key)
			h.OnDelete(key)
		default:
			return fmt.Errorf("invalid event type: %s event: %+v", ev.Type, ev)
		}
	}
	return nil
}

func (e *Etcd) fetchServers(fullKey string) ([]*server.Server, error) {
	client := e.client
	if client == nil {
		return nil, fmt.Errorf("nil etcd client")
	}
	key := fullKey
	ctx, cancel := context.WithTimeout(context.TODO(), 6*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil || len(resp.Kvs) <= 0 {
		return nil, err
	}

	slist := []*server.Server{}
	for _, pair := range resp.Kvs {
		data := pair.Value
		s := server.Server{}
		if err := json.Unmarshal(data, &s); err != nil {
			log2.Warnf("fetch servers failed: invalid data: key=%s value:%v err:%v", pair.Key, string(data), err)
			continue
		}
		if !s.IsValid() {
			log2.Warnf("fetch servers failed: invalid data: key=%s value:%v", pair.Key, string(data))
			continue
		}
		s.SetKey(string(pair.Key))
		slist = append(slist, &s)
	}
	return slist, nil
}

func (e *Etcd) fetchServersAlived(fullKey string, hc health.Checker) ([]*server.Server, error) {
	servers, err := e.fetchServers(fullKey)
	if err != nil {
		return nil, err
	}
	alives, _ := server.Filter(servers, hc)
	return alives, nil
}

func (e *Etcd) newLeagueID(ttl time.Duration) (clientv3.LeaseID, error) {
	lease := clientv3.NewLease(e.client)
	ttlSecs := int64(ttl / time.Second)
	resp, err := lease.Grant(context.TODO(), ttlSecs)
	if err != nil {
		return 0, err
	}
	return resp.ID, nil
}

func (e *Etcd) Start(ctx context.Context, s *server.Server, hooks ...broker.Hook) error {
	if !s.IsValid() {
		return fmt.Errorf("invalid server: %+v", s)
	}
	key := e.buildKey(s.Kind, s.ID)
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	s.SetStatus(server.States.Running)
	ttl := e.ttl
	if ttl < 30*time.Second {
		ttl = 30 * time.Second
	}
	leaseId, err := e.newLeagueID(ttl)
	if err != nil {
		return err
	}
	_, err = e.client.Put(ctx, key, string(data), clientv3.WithLease(leaseId))
	if err != nil {
		log2.Warnf("[etcd] server start failed!!!. key=%s err:%v", key, err)
		return err
	}

	keepaliveCh, err := e.client.KeepAlive(ctx, leaseId)
	if err != nil {
		return err
	}

	log2.Infof("[etcd] server started. key=%s", key)
	go func() {
		for {
			select {
			case <-keepaliveCh:
				if len(hooks) > 0 {
					hooks[0](s)
				}
				s.UpdatedAt = time.Now()
				if err := e.update(s, &leaseId); err != nil {
					log2.Warnf("[etcd] server update failed!! err:%v", err)
				}
			case state := <-e.stateCh:
				s.SetStatus(state)
				s.UpdatedAt = time.Now()
				if err := e.update(s, &leaseId); err != nil {
					log2.Warnf("[etcd] server update failed!!. err:%v", err)
				}
			case <-ctx.Done():
				if err := e.stop(s); err != nil {
					log2.Infof("[etcd] server stopped. key=%s but err:%v", key, err)
				} else {
					log2.Infof("[etcd] server stopped. key=%s", key)
				}
				return
			}
		}
	}()
	return nil
}

func (e *Etcd) update(s *server.Server, leaseId *clientv3.LeaseID) error {
	if !s.IsValid() {
		return fmt.Errorf("invalid server: %+v", s)
	}
	key := e.buildKey(s.Kind, s.ID)
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.TODO(), 6*time.Second)
	defer cancel()

	_, err = e.client.Put(ctx, key, string(data), clientv3.WithLease(*leaseId))
	if err != nil {
		log2.Warnf("[etcd] server keepalive failed!!!. key=%s err:%v", key, err)
		_leaseId, err := e.newLeagueID(30 * time.Second)
		if err != nil {
			log2.Errorf("[etcd] server update failed!!!. key=%s err:%v", key, err)
			return err
		}
		*leaseId = _leaseId
		_, err = e.client.Put(ctx, key, string(data), clientv3.WithLease(*leaseId))
		if err != nil {
			log2.Errorf("[etcd] server update failed!!!. key=%s err:%v", key, err)
			return err
		}
		return err
	}
	return err
}

func (e *Etcd) stop(s *server.Server) error {
	if !s.IsValid() {
		return fmt.Errorf("invalid server: %+v", s)
	}
	ctx, cancel := context.WithTimeout(context.TODO(), 6*time.Second)
	defer cancel()

	key := e.buildKey(s.Kind, s.ID)
	_, err := e.client.Delete(ctx, key)
	if err != nil {
		log2.Errorf("[etcd] server stop failed. key=%s err:%v ", key, err)
		return err
	}
	log2.Infof("[etcd] server stopped. key=%s", key)
	return nil
}

func (e *Etcd) SetState(state server.State) {
	e.stateCh <- state
}

func (e *Etcd) buildKey(kind, id string) string {
	return strings.Join([]string{e.baseKey, kind, id}, "/")
}

func (e *Etcd) buildKeyOfList(kind string) string {
	return strings.Join([]string{e.baseKey, kind}, "/") + "/"
}

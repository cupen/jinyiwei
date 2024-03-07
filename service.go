package xdisco

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cupen/xdisco/broker"
	"github.com/cupen/xdisco/eventhandler"
	"github.com/cupen/xdisco/server"
	"go.uber.org/zap"
)

type Service struct {
	kind      string
	m         sync.Map
	healths   atomic.Value // ServerList
	unhealths atomic.Value // ServerList Unhealth
	broker    broker.Broker
	checker   server.Checker
	onChanged func(*Service)

	logPrefix string
}

func NewService(kind string, w broker.Broker, c server.Checker) *Service {
	if w == nil {
		panic(fmt.Errorf("nil broker"))
	}
	if c == nil {
		panic(fmt.Errorf("nil health checker"))
	}
	obj := Service{
		kind:    kind,
		broker:  w,
		checker: c,
	}
	return &obj
}

func (this *Service) Start(ctx context.Context) error {
	err := this.broker.Watch(ctx, this.kind, this.Handler(), this.checker)
	if err != nil {
		log.Warn("[service] watch failed", zap.Error(err))
	}
	return err
}

func (this *Service) Kind() string {
	return this.kind
}

func (this *Service) Checker() server.Checker {
	return this.checker
}

func (this *Service) Handler() eventhandler.Handler {
	return eventhandler.Handler{
		OnInit:   this.onServersInit,
		OnAdd:    this.onServerAdd,
		OnUpdate: this.onServerUpdate,
		OnDelete: this.onServerDelete,
	}
}

func (this *Service) OnChanged(callback func(*Service)) {
	this.onChanged = callback
}

func (this *Service) ChooseServer(id string) *server.Server {
	servers := this.GetServerList()
	return servers.Lookup(id)
}

func (this *Service) GetServerList() *server.ServerList {
	servers := this.healths.Load().(*server.ServerList)
	return servers
}

func (this *Service) buildServerList(list []*server.Server) {
	for _, s := range list {
		this.m.Store(s.GetKey(), s)
	}
	this.renewServers()
}

func (this *Service) renewServers() {
	serverlist := server.NewServerListFromMap(&this.m)
	this.healths.Store(serverlist)
}

func (this *Service) onServersInit(servers []*server.Server) {
	alives, dead := server.Filter(servers, this.checker)
	for _, s := range alives {
		key := s.GetKey()
		this.m.Store(key, s)
		log2.Infof("server<%s> initialized: %s", s.Kind, key)
	}
	this.renewServers()
	if this.onChanged != nil {
		this.onChanged(this)
	}
	for _, s := range dead {
		key := s.GetKey()
		this.onServerUnhealth(key, s)
		log2.Warnf("server<%s> unhealth: key=%s", s.Kind, key)
	}
}

func (this *Service) onServerAdd(key string, s *server.Server) {
	now := time.Now()
	if err := s.Check(this.checker); err != nil {
		log2.Warnf("server<%s> unhealth: %s reason:%v", key, err)
		this.onServerUnhealth(key, s)
		return
	}
	this.m.Store(key, s)
	this.renewServers()
	log2.Infof("server<%s> found  : %s  cost: %v", s.Kind, key, time.Since(now))
	if this.onChanged != nil {
		this.onChanged(this)
	}
}

func (this *Service) onServerUpdate(key string, s *server.Server) {
	now := time.Now()
	if err := s.Check(this.checker); err != nil {
		log2.Warnf("server<%s> unhealth: %s err: %s", s.Kind, key, err)
		this.onServerUnhealth(key, s)
		return
	}
	this.m.Store(key, s)
	this.renewServers()
	log2.Debugf("server<%s> alives: %s  cost: %v", s.Kind, key, time.Since(now))
	if this.onChanged != nil {
		this.onChanged(this)
	}
}

func (this *Service) onServerDelete(key string) {
	now := time.Now()
	this.m.Delete(key)
	this.renewServers()
	log2.Infof("server<%s> deleted: %s  cost: %v", this.kind, key, time.Since(now))
	if this.onChanged != nil {
		this.onChanged(this)
	}
}

func (this *Service) onServerUnhealth(key string, s *server.Server) {

}

func (this *Service) CleanUnhealthServer() (deleted int, isChanged bool) {
	servers := this.GetServerList().GetAll()
	if len(servers) <= 0 {
		return
	}
	alives, deads := server.Filter(servers, this.checker)
	if len(alives) != len(servers) {
		for _, dead := range deads {
			this.m.Delete(dead.GetKey())
			// this.onServerUnhealth(dead.GetKey(), dead)
		}
		this.renewServers()

		// server.Sort(deads)
		// this.unhealths.Store(server.NewServerList(deads))
		log2.Warnf("serverlist<%s> cleaned. changed: %d -> %d", this.Kind, len(servers), len(alives))
		return len(deads), true
	}
	return len(deads), false
}

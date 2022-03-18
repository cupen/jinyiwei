package server

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cupen/xdisco/health"
)

type Server struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Addr        string            `json:"addr"`
	PublicAddr  string            `json:"publicAddr"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Status      string            `json:"status"`
	Weight      int               `json:"weight"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	CreatedAt   time.Time         `json:"createdAt"`
	key         string            `json:"-"`
}

func NewServer(id, kind string, addr string) *Server {
	now := time.Now()
	return &Server{
		ID:        id,
		Kind:      kind,
		Addr:      addr,
		UpdatedAt: now,
		CreatedAt: now,
		Labels:    map[string]string{},
		key:       fmt.Sprintf("%s/%s", kind, id),
	}
}

func NewServerByBytes(data []byte) (*Server, error) {
	s := Server{}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s-%s", s.Kind, s.ID)
	s.SetKey(key)
	return &s, nil
}

func NewServerFromEtcd(key string, data []byte) (*Server, error) {
	s := Server{}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	s.SetKey(key)
	return &s, nil
}

func (s *Server) GetID() string {
	return s.ID
}

func (s *Server) IsValid() bool {
	return s.Kind != "" && s.ID != ""
}

func (s *Server) GetLabel(key string) string {
	if len(s.Labels) <= 0 {
		return ""
	}
	if val, ok := s.Labels[key]; ok {
		return val
	}
	return ""
}

func (s *Server) Check(hc health.Checker) error {
	if hc == nil {
		return fmt.Errorf("nil healch checker")
	}
	return hc.Ping(s.Addr, s.PublicAddr)
}

func (s *Server) SetKey(key string) {
	s.key = key
}

func (s *Server) GetKey() string {
	return s.key
}

func (s *Server) SetStatus(ss State) {
	s.Status = string(ss)
}

func (s *Server) GetStatus() State {
	return State(s.Status)
}

func (s *Server) SetAnnotation(name, value string) {
	if s.Annotations == nil {
		s.Annotations = map[string]string{}
	}
	s.Annotations[name] = value
}

func (s *Server) GetAnnotation(key string) string {
	if len(s.Annotations) <= 0 {
		return ""
	}
	v, _ := s.Annotations[key]

	return v
}

func (s *Server) GetAnnotationAsInt(key string, defaultVal int) int {
	if len(s.Annotations) <= 0 {
		return defaultVal
	}
	if v, ok := s.Annotations[key]; ok {
		rs, _ := strconv.Atoi(v)
		return rs
	}
	return defaultVal
}

func Filter(list []*Server, hc health.Checker) (alives []*Server, deads []*Server) {
	var wg sync.WaitGroup
	var checkOK sync.Map
	var checkFail sync.Map
	for _, s := range list {
		wg.Add(1)
		var local_s = s
		go func() {
			defer wg.Done()
			if err := local_s.Check(hc); err == nil {
				checkOK.Store(local_s.ID, local_s)
			} else {
				log2.Infof("health check fail. key:%s err:%v", local_s.GetKey(), err)
				checkFail.Store(local_s.ID, local_s)
			}
		}()
	}
	wg.Wait()
	checkOK.Range(func(k, v interface{}) bool {
		alives = append(alives, v.(*Server))
		return true
	})

	checkFail.Range(func(k, v interface{}) bool {
		deads = append(deads, v.(*Server))
		return true
	})
	return

}

package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	ID    string         `json:"id"`
	Kind  string         `json:"kind"`
	Host  string         `json:"host"`
	Ports map[string]int `json:"ports"`

	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Status      string            `json:"status"`
	Weight      int               `json:"weight"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	CreatedAt   time.Time         `json:"createdAt"`
	key         string            `json:"-"`
}

func NewServer(id, kind string, host string) *Server {
	now := time.Now()
	return &Server{
		ID:          id,
		Kind:        kind,
		Host:        host,
		UpdatedAt:   now,
		CreatedAt:   now,
		Labels:      map[string]string{},
		Ports:       map[string]int{},
		Annotations: map[string]string{},
		key:         fmt.Sprintf("%s/%s", kind, id),
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

func (s *Server) PrivateAddress(portName string) string {
	return fmt.Sprintf("%s:%d", s.Host, s.Ports[portName])
}

func (s *Server) LocalAddress(portName string) string {
	return fmt.Sprintf("127.0.0.1:%d", s.Ports[portName])
}

func (s *Server) PublicAddress(portName string) string {
	return fmt.Sprintf("0.0.0.0:%d", s.Ports[portName])
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

func (s *Server) Check(hc Checker) error {
	if hc == nil {
		return fmt.Errorf("nil healch checker")
	}
	return hc.Ping(s)
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

func (s *Server) GetAnnotation(key string) (string, bool) {
	if len(s.Annotations) <= 0 {
		return "", false
	}
	v, ok := s.Annotations[key]
	return v, ok
}

func (s *Server) GetAnnotationAsInt(key string, defaultVal int) (int, error) {
	if len(s.Annotations) <= 0 {
		return defaultVal, nil
	}
	if v, ok := s.Annotations[key]; ok {
		rs, err := strconv.Atoi(v)
		if err != nil {
			return defaultVal, err
		}
		return rs, nil
	}
	return defaultVal, nil
}

func Filter(list []*Server, hc Checker) (alives []*Server, deads []*Server) {
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

func Sort(list []*Server) {
	if len(list) <= 1 {
		return
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.Compare(list[i].GetID(), list[j].GetID()) < 0
	})
}

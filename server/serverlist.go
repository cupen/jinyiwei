package server

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/cupen/xdisco/lookup"
)

type ServerList struct {
	chash      lookup.Lookup
	serverMap  map[string]*Server
	serverList []*Server // sorted
}

func NewServerList(list []*Server) *ServerList {
	m := map[string]*Server{}
	for _, s := range list {
		m[s.GetID()] = s
	}

	buckets := make([]string, len(list))
	for i := 0; i < len(list); i++ {
		buckets[i] = list[i].Addr
	}
	return &ServerList{
		chash:      lookup.NewRendezvous(buckets),
		serverMap:  m,
		serverList: list,
	}
}

func NewServerListFromMapV2(m map[string]*Server) *ServerList {
	list := make([]*Server, len(m))
	i := 0
	for _, s := range m {
		list[i] = s
		i++
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.Compare(list[i].GetID(), list[j].GetID()) < 0
	})
	return NewServerList(list)
}

func NewServerListFromMap(m *sync.Map) *ServerList {
	list := []*Server{}
	m.Range(func(originKey, originValue interface{}) bool {
		// key := originKey.(string)
		s := originValue.(*Server)
		list = append(list, s)
		return true
	})

	sort.Slice(list, func(i, j int) bool {
		return strings.Compare(list[i].GetID(), list[j].GetID()) < 0
	})
	return NewServerList(list)
}

func (this *ServerList) Has(sid string) bool {
	_, ok := this.serverMap[sid]
	return ok
}

func (this *ServerList) Get(sid string) *Server {
	if val, ok := this.serverMap[sid]; ok {
		return val
	}
	return nil
}

func (this *ServerList) ForEach(f func(string, *Server)) {
	for sid, s := range this.serverMap {
		f(sid, s)
	}
}

func (this *ServerList) GetAll() []*Server {
	return this.serverList
}

func (this *ServerList) GetMap() map[string]*Server {
	return this.serverMap
}

func (this *ServerList) Size() int {
	return len(this.serverList)
}

func (this *ServerList) GetByLabels(filter map[string]string) []*Server {
	rs := []*Server{}
	for _, s := range this.serverList {
		for k, v := range filter {
			if v == s.GetLabel(k) {
				rs = append(rs, s)
			}
		}
	}
	return rs
}

func (this *ServerList) Dump() string {
	lines := []string{"========================= serverlist ========================="}
	for _, s := range this.GetAll() {
		lines = append(lines, fmt.Sprintf("%s\n\t%+v", s.GetKey(), s))
	}
	if len(lines) <= 1 {
		return "empty"
	}
	lines = append(lines, "===============================================================")
	return "\n" + strings.Join(lines, "\n")
}

func (this *ServerList) Lookup(id string) *Server {
	serverId := this.chash.Get(id)
	return this.Get(serverId)
}

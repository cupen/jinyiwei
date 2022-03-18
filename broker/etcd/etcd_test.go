package etcd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/cupen/xdisco/eventhandler"
	"github.com/cupen/xdisco/health"
	"github.com/cupen/xdisco/server"
	"github.com/stretchr/testify/assert"
)

func TestWatch_Init(t *testing.T) {
	assert := assert.New(t)

	bk, err := New("/testcase/watch/j&jio(llg", 10*time.Second)
	assert.NoError(err)
	hchecker := health.NewFunc(func(addr, publicAddr string) error {
		return nil
	})

	ctx := context.TODO()
	// start server before
	serverCount := 10
	servers := make([]*server.Server, serverCount)
	for i := 0; i < serverCount; i++ {
		servers[i] = newServer(i + 1)
		servers[i].SetKey(bk.baseKey + "/" + servers[i].GetKey())
		servers[i].Status = "running"
		bk.Start(ctx, servers[i])
	}
	t.Cleanup(func() {
		for _, s := range servers {
			bk.stop(s)
		}
	})

	serversReceived := map[string]*server.Server{}
	ctx, cancel := context.WithCancel(context.TODO())
	handler := eventhandler.Handler{
		OnInit: func(servers []*server.Server) {
			for _, s := range servers {
				serversReceived[s.GetKey()] = s
			}
			if len(serversReceived) >= 10 {
				cancel()
			}
		},
		OnAdd: func(key string, s *server.Server) {
			serversReceived[key] = s
			if len(serversReceived) >= 10 {
				cancel()
			}
		},
		OnUpdate: func(key string, s *server.Server) {
			serversReceived[key] = s
			if len(serversReceived) >= 10 {
				cancel()
			}
		},
		OnDelete: func(key string) {
			delete(serversReceived, key)
			if len(serversReceived) >= 10 {
				cancel()
			}
		},
	}

	assert.True(handler.IsValid())
	err = bk.Watch(ctx, servers[0].Kind, handler, hchecker)
	assert.NoError(err)
	select {
	case <-ctx.Done():
		sl := server.NewServerListFromMapV2(serversReceived)
		serversInited := sl.GetAll()
		if assert.Equal(len(servers), len(serversInited)) {
			sort.Slice(serversInited, func(i, j int) bool {
				I, _ := strconv.Atoi(serversInited[i].ID)
				J, _ := strconv.Atoi(serversInited[j].ID)
				return I < J
			})
			for i := 0; i < len(servers); i++ {
				servers[i].CreatedAt = serversInited[i].CreatedAt
				servers[i].UpdatedAt = serversInited[i].UpdatedAt
				assert.Equalf(servers[i], serversInited[i], "i=%d", i)
			}
			// assert.Equal(servers, serversInited)
		}
	case <-time.After(3 * time.Second):
		assert.FailNow("timeout")
	}
}

func newServer(id int) *server.Server {
	addr := fmt.Sprintf("127.0.0.1:%d", id)
	return server.NewServer(strconv.Itoa(id), "usercase01", addr)
}

package server

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer(_t *testing.T) {
	newList := func(count int) []*Server {
		rs := []*Server{}
		for i := 0; i < count; i++ {
			rs = append(rs, &Server{
				ID:   fmt.Sprintf("%d", i),
				Addr: fmt.Sprintf("127.0.0.1:%d", i),
			})
		}
		return rs
	}

	newMap := func(count int) *sync.Map {
		var m = sync.Map{}
		for _, s := range newList(count) {
			m.Store(s.GetID(), s)
		}
		return &m
	}

	_t.Run("newByMap", func(t *testing.T) {
		assert := assert.New(t)
		assert.Equal(0, 0)
		obj := NewServerListFromMap(newMap(10))
		assert.Equal(newList(10), obj.GetAll())
	})
}

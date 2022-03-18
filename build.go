package xdisco

import (
	"fmt"

	"github.com/cupen/xdisco/server"
)

func NewServer(kind, addr string) *server.Server {
	if kind == "" {
		panic(fmt.Errorf("empty kind of server"))
	}
	if addr == "" {
		panic(fmt.Errorf("empty address of server"))
	}
	id := fmt.Sprintf("%s", addr)
	return server.NewServer(id, kind, addr)
}

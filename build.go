package xdisco

import (
	"fmt"

	"github.com/cupen/xdisco/server"
)

func NewServer(id, kind, host string) *server.Server {
	if kind == "" {
		panic(fmt.Errorf("empty kind of server"))
	}
	if host == "" {
		panic(fmt.Errorf("empty address of server"))
	}
	return server.NewServer(id, kind, host)
}

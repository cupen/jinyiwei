package broker

import (
	"context"

	"github.com/cupen/xdisco/eventhandler"
	"github.com/cupen/xdisco/health"
	"github.com/cupen/xdisco/server"
)

type Hook func(*server.Server)

type Broker interface {
	Wacher
	Sevice
}

// Watcher ...
type Wacher interface {
	Watch(context.Context, string, eventhandler.Handler, health.Checker) error
}

// Service ...
type Sevice interface {
	Start(context.Context, *server.Server, ...Hook) error
	SetState(server.State)
}

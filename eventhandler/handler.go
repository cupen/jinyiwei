package eventhandler

import "github.com/cupen/xdisco/server"

type Handler struct {
	OnInit   func([]*server.Server)
	OnAdd    func(key string, s *server.Server)
	OnUpdate func(key string, s *server.Server)
	OnDelete func(key string)
}

func (h *Handler) IsValid() bool {
	return h.OnInit != nil && h.OnAdd != nil &&
		h.OnUpdate != nil && h.OnDelete != nil
}

package health

import "github.com/cupen/xdisco/server"

type customChecker struct {
	pinger func(*server.Server) error
}

func Custom(f func(*server.Server) error) *customChecker {
	return &customChecker{
		pinger: f,
	}
}

func (fw *customChecker) Ping(s *server.Server) error {
	return fw.pinger(s)
}

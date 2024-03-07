package health

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cupen/xdisco/server"
)

const (
	TIMEOUT = 2 * time.Second
	RETRIES = 3
)

type httpChecker struct {
	Timeout  time.Duration
	Retries  int
	method   string
	path     string
	portName string
}

func Http(method, path string, portName ...string) *httpChecker {
	method = strings.ToUpper(method)
	switch method {
	case "GET", "HEAD", "OPTIONS":
	default:
		panic(fmt.Errorf("unsupported method: %s", method))
	}
	var _portName = "http"
	if len(portName) > 0 {
		_portName = portName[0]
	}
	return &httpChecker{
		Timeout:  TIMEOUT,
		Retries:  RETRIES,
		method:   method,
		path:     path,
		portName: _portName,
	}
}

func (hh *httpChecker) wrapError(url string, resp *http.Response, err error) error {
	if err != nil {
		return fmt.Errorf("%w from %s", err, url)
	}
	if resp == nil {
		return fmt.Errorf("no response from '%s'", url)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("non-200 status[%d] from '%s'", resp.StatusCode, url)
	}
	return nil
}

func (hh *httpChecker) Ping(s *server.Server) error {
	addr := s.PrivateAddress(hh.portName)
	if addr == "" {
		return fmt.Errorf("empty address, portName:%s", hh.portName)
	}
	var timeout = hh.Timeout
	var retries = hh.Retries
	var lastErr error
	req := http.Client{Timeout: timeout}
	url := fmt.Sprintf("http://%s/health/status", addr)
	resp, err := req.Head(url)
	lastErr = hh.wrapError(url, resp, err)
	if lastErr == nil {
		// ping success!
		return nil
	}
	for i := 0; i < retries-1; i++ {
		resp, err := req.Head(url)
		if lastErr = hh.wrapError(url, resp, err); lastErr == nil {
			// ping success!
			return nil
		}
	}
	return lastErr
}

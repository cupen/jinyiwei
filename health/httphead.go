package health

import (
	"fmt"
	"net/http"
	"time"
)

const (
	TIMEOUT = 1 * time.Second
	RETRIES = 3
)

type HttpHead struct {
	Timeout time.Duration
	Retries int
}

func NewHttpHead() *HttpHead {
	return &HttpHead{
		Timeout: TIMEOUT,
		Retries: RETRIES,
	}
}

func (hh *HttpHead) wrapError(url string, resp *http.Response, err error) error {
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

func (hh *HttpHead) Ping(addr, publicUrl string) error {
	if addr == "" {
		return fmt.Errorf("empty address")
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

package etcd

import (
	"fmt"
	"time"

	// "github.com/coreos/etcd/clientv3"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Options struct {
	// keyspace
	BaseKey string `json:"baseKey" toml:"baseKey" validate:"required"`

	// etcd server endpoints
	Endpoints []string `json:"endpoints" toml:"endpoints" validate:"required"`

	// client connection timeout
	Timeout time.Duration `json:"timeout" toml:"timeout" validate:"required"`

	// server lease ttl
	TTL time.Duration `json:"ttl" toml:"ttl" validate:"required"`
}

func (c *Options) CheckBasic() error {
	if c.BaseKey == "" {
		return fmt.Errorf("empty baseKey: %s", c.BaseKey)
	}
	if c.TTL <= 0 {
		return fmt.Errorf("invalid ttl: %v", c.TTL)
	}
	return nil
}

func (c *Options) Check() error {
	if err := c.CheckBasic(); err != nil {
		return err
	}
	if len(c.Endpoints) <= 0 {
		return fmt.Errorf("empty endpoints: %v", c.Endpoints)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("invalid timeout: %v", c.Timeout)
	}
	return nil
}

func (c *Options) WithDefault() *Options {
	defaultConfig := DefaultOptions()
	if c.BaseKey == "" {
		c.BaseKey = defaultConfig.BaseKey
	}
	if len(c.Endpoints) <= 0 {
		c.Endpoints = defaultConfig.Endpoints
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultConfig.Timeout
	}
	if c.TTL <= 0 {
		c.TTL = defaultConfig.TTL
	}
	// if err := c.Check(); err != nil {
	// 	panic(err)
	// }
	return c
}

func (c *Options) EtcdConfig() clientv3.Config {
	c = c.WithDefault()
	return clientv3.Config{
		Endpoints:   c.Endpoints,
		DialTimeout: c.Timeout,
	}
}

func DefaultOptions() *Options {
	return &Options{
		BaseKey:   "/xdisco",
		Endpoints: []string{"127.0.0.1:2379"},
		Timeout:   5 * time.Second,
		TTL:       10 * time.Second,
	}
}

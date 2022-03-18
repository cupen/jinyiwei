package xdisco

import (
	"fmt"

	// "github.com/coreos/etcd/clientv3"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Config struct {
	BaseKey string          `json:"baseKey" toml:"baseKey"`
	Config  clientv3.Config `json:"config" toml:"config"`
}

func (c *Config) Check() error {
	if c.BaseKey == "" {
		return fmt.Errorf("Empty baseKey: %s", c.BaseKey)
	}
	if len(c.Config.Endpoints) <= 0 {
		return fmt.Errorf("Empty endpoints: %s", c.Config.Endpoints)
	}
	return nil
}

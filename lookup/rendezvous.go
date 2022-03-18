package lookup

import (
	xxhash "github.com/cespare/xxhash/v2"
	rendezvous "github.com/dgryski/go-rendezvous"
)

type Rendezvous struct {
	rdz *rendezvous.Rendezvous
}

func NewRendezvous(buckets []string) *Rendezvous {
	return &Rendezvous{
		rdz: rendezvous.New(buckets, xxhash.Sum64String),
	}
}

func (r *Rendezvous) Get(key string) string {
	return r.rdz.Lookup(key)
}

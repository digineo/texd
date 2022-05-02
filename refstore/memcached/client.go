package memcached

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/bradfitz/gomemcache/memcache"
)

type client interface {
	Get(key string) (*memcache.Item, error)
	Set(*memcache.Item) error
	Touch(key string, seconds int32) error
}

func newClient(host string, params url.Values) (client, error) {
	var hosts []string
	if host != "" {
		hosts = append(hosts, host)
	}
	hosts = append(hosts, params["addr"]...)
	if len(hosts) == 0 {
		return nil, errors.New("no server(s) configured")
	}
	client := memcache.New(hosts...)

	if v := params.Get("timeout"); v != "" {
		t, err := parseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout parameter: %w", err)
		}
		if t < 0 {
			return nil, errors.New("negative timeout parameter")
		}
		client.Timeout = t
	} else {
		client.Timeout = DefaultTimeout
	}

	if v := params.Get("max_idle_conns"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid max_idle_conns parameter: %w", err)
		}
		if n < 0 {
			return nil, errors.New("negative max_idle_conns parameter")
		}
		client.MaxIdleConns = n
	} else {
		client.MaxIdleConns = DefaultMaxIdleConns
	}

	return client, nil
}

func (s *store) get(key string) ([]byte, error) {
	it, err := s.client.Get(key)
	if err != nil {
		return nil, err
	}
	if exp := s.expiration(); exp > 0 {
		go func() { _ = s.client.Touch(key, exp) }()
	}
	return it.Value, nil
}

func (s *store) expiration() int32 {
	if s.expire == 0 {
		return 0
	}
	if s.expire < relativeExpirationThreshold {
		return int32(s.expire.Seconds())
	}
	return int32(s.clock.Now().Add(s.expire).Unix())
}

// package memcached implements a reference store adapter backed by a
// separate Memcached server.
//
// To be able to use it, add an anonymous import to your main package:
//
//	import _ "github.com/digineo/texd/refstore/memcached"
//
// This registers the "memcached://" adapter.
//
// For configuration, use a DSN with the following shape:
//
//	dsn := "memcached://host?options"
//	dir, _ := refstore.NewStore(dsn, nil)
//
// See New() for available options.
//
// Note: This adapter ignores any retention policy. It is expected to
// configure the Memcached server instance accordingly.
package memcached

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/digineo/texd/internal"
	"github.com/digineo/texd/refstore"
	"go.uber.org/zap"
)

// Defaults copied from github.com/bradfitz/gomemcache/memcache for clarity
const (
	DefaultMaxIdleConns = memcache.DefaultMaxIdleConns
	DefaultTimeout      = memcache.DefaultTimeout
)

const (
	// MaxExpiration is the larges value supported for the expiration=<duration>
	// config parameter.
	MaxExpiration = time.Duration(math.MaxInt32) * time.Second

	// DefaultExpiration is used when no expiration=<duration> config parameter
	// is given. This disables automatic expiration.
	DefaultExpiration = 0

	// implementation detail: the Memcached protocol interprets expiry values
	// larger than relativeExpirationThreshold as absolute timestamps.
	relativeExpirationThreshold = 30 * 24 * time.Hour

	// DefaultKeyPrefix is used with file reference IDs to lookup and store
	// data in Memcached.
	DefaultKeyPrefix = "texd/"
)

func init() {
	refstore.RegisterAdapter("memcached", New)
}

type store struct {
	client    client
	keyPrefix string
	expire    time.Duration
	clock     internal.Clock
}

// New configures a memcached storage adapter. The retention policy is
// ignored, any data retention and cache invalidation strategy is delegated
// to the Memcached server.
//
// The following URI parameters are understood (any similarity with
// github.com/bradfitz/gomemcache/memcache is intentional):
//
//	- addr=<host> adds an additional server to the pool. This option can
//	  be specified multiple times. If an address is specified multiple times,
//	  it gains a proportional amount of weight.
//	  Note: you may omit config.Host, and only use addr=<host> URI parameters.
//	- timeout=<duration> to specify the read/write timeout. The parameter
//	  value will be parsed with time.ParseDuration(). Values < 0 are invalid
//	  (New() will return an error) and the zero value is substituted with
//	  a default (100ms).
//	- max_idle_conns=<num> specifies the maximum number of idle connections
//	  that will be maintained per address. Negative values are invalid,
//	  and the zero value is substituted with a default (2).
//	- expiration=<duration> adds a lifetime to reference files. A value <= 0
//	  is ignored (no error). The duration must fit into a int32, which
//	  imposes a maximum duration of just over 68 years.
//	- key_prefix=<string> adds a custom prefix to file reference hashes.
//	  By default, this adapter prefixes reference hashes with "texd/".
//
// Note that server addresses (whether provided as DNS host name or via query
// parameters) should include a port number (by default, Memcached listens
// on port 11211).
func New(config *url.URL, _ refstore.RetentionPolicy) (refstore.Adapter, error) {
	q := config.Query()

	client, err := newClient(config.Host, q)
	if err != nil {
		return nil, fmt.Errorf("memcached: %w", err)
	}

	adapter := &store{
		client:    client,
		expire:    DefaultExpiration,
		keyPrefix: DefaultKeyPrefix,
		clock:     internal.SystemClock{},
	}

	if v := q.Get("expiration"); v != "" {
		t, err := parseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("memcached: invalid expiration parameter: %w", err)
		}
		if t > MaxExpiration {
			return nil, fmt.Errorf("memcached: expiration parameter too large, must be <= %s", MaxExpiration)
		}
		if t > 0 {
			adapter.expire = t
		}
	}

	if v := q.Get("key_prefix"); v != "" {
		if maxLen := 250 - 43; len(v) > maxLen {
			return nil, fmt.Errorf("memcached: key_prefix parameter must be <= %d characters", maxLen)
		}
		adapter.keyPrefix = v
	}

	return adapter, nil
}

func (s *store) CopyFile(_ *zap.Logger, id refstore.Identifier, w io.Writer) error {
	val, err := s.get(s.key(id))
	if err != nil {
		if errors.Is(err, memcache.ErrCacheMiss) {
			return refstore.ErrUnknownReference
		}
		return fmt.Errorf("memcached: failed to retreive storage object: %w", err)
	}

	if _, err = io.Copy(w, bytes.NewReader(val)); err != nil {
		return fmt.Errorf("memcached: failed to copy storage object: %w", err)
	}
	return nil
}

func (s *store) Store(_ *zap.Logger, r io.Reader) error {
	val, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("memcached: failed to read reference file: %w", err)
	}
	id := refstore.NewIdentifier(val)
	err = s.client.Set(&memcache.Item{
		Key:        s.key(id),
		Value:      val,
		Expiration: s.expiration(),
	})
	if err != nil {
		return fmt.Errorf("memcached: failed to create storage object: %w", err)
	}
	return nil
}

func (s *store) Exists(id refstore.Identifier) bool {
	_, err := s.get(s.key(id))
	return err == nil
}

func (s *store) key(id refstore.Identifier) string {
	return s.keyPrefix + id.Raw()
}

func parseDuration(s string) (time.Duration, error) {
	// try plain number conversion first
	if val, err := strconv.Atoi(s); err == nil {
		return time.Duration(val) * time.Second, nil
	}
	// otherwise use "proper" duration parsing
	return time.ParseDuration(s)
}

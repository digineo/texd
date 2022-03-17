package refstore

import (
	"fmt"
	"net/url"
	"sort"
	"sync"
)

type AdapterConstructor func(*url.URL) (Adapter, error)

type ErrStoreAlreadyTaken string

func (err ErrStoreAlreadyTaken) Error() string {
	return fmt.Sprintf("refstore: the name %q is already taken by another adapter package", string(err))
}

var (
	stores  = map[string]AdapterConstructor{}
	storeMu = sync.RWMutex{}
)

// RegisterAdapter will remember the given adapter with under the
// given name. It will panic, if the name is already taken.
func RegisterAdapter(name string, adapter AdapterConstructor) {
	storeMu.Lock()
	defer storeMu.Unlock()

	if _, taken := stores[name]; taken {
		panic(ErrStoreAlreadyTaken(name))
	}
	stores[name] = adapter
}

// NewStore creates a new reference store with the given DSN. The adapter
// name is extracted from the DSN, i.e. the disk adapter requires a
// DSN of the form "disk://".
func NewStore(dsn string) (Adapter, error) {
	storeMu.RLock()
	defer storeMu.RUnlock()

	uri, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN: %v", err)
	}

	constructor, exists := stores[uri.Scheme]
	if !exists {
		return nil, fmt.Errorf("unknown storage adapter %q", uri.Scheme)
	}

	return constructor(uri)
}

func AvailableAdapters() (list []string) {
	storeMu.RLock()
	defer storeMu.RUnlock()

	for name := range stores {
		list = append(list, name)
	}
	sort.Strings(list)
	return
}

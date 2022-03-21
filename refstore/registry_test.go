package refstore

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapters(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	assert.Len(AvailableAdapters(), 0)

	var gotURL *url.URL
	var gotRP RetentionPolicy
	stores["dummy"] = func(u *url.URL, rp RetentionPolicy) (Adapter, error) {
		gotURL = u
		gotRP = rp
		return nil, nil
	}
	defer delete(stores, "dummy")

	assert.EqualValues(AvailableAdapters(), []string{"dummy"})

	a, err := NewStore("dummy://dummy", nil)
	require.NoError(err)
	assert.Nil(a)

	require.NotNil(gotURL)
	assert.Equal("dummy://dummy", gotURL.String())

	require.NotNil(gotRP)
	_, ok := gotRP.(*KeepForever)
	assert.True(ok)
}

func TestRegistry(t *testing.T) {
	defer func() {
		stores = make(map[string]AdapterConstructor)
	}()

	dummy := func(u *url.URL, rp RetentionPolicy) (Adapter, error) {
		return nil, nil
	}

	// adding adapter with same name panics
	require.NotPanics(t, func() { RegisterAdapter("dummy", dummy) })
	require.Panics(t, func() { RegisterAdapter("dummy", dummy) })

	// using a different name is OK
	require.NotPanics(t, func() { RegisterAdapter("dummy2", dummy) })
}

func TestNewStore_invalidDSN(t *testing.T) {
	s, err := NewStore(":", nil)
	assert.Nil(t, s)
	assert.EqualError(t, err, `invalid DSN: parse ":": missing protocol scheme`)
}

func TestNewStore_unknownAdapter(t *testing.T) {
	assert.Len(t, stores, 0)

	s, err := NewStore("foo://", nil)
	assert.Nil(t, s)
	assert.EqualError(t, err, `unknown storage adapter "foo"`)
}

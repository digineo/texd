package memcached

import (
	"bytes"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/digineo/texd/internal"
	"github.com/digineo/texd/refstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestInit(t *testing.T) {
	adapter, err := refstore.NewStore("memcached://host", nil)
	require.NoError(t, err)
	require.NotNil(t, adapter)

	_, ok := adapter.(*store)
	require.True(t, ok)
}

func TestNew_validConfig(t *testing.T) {
	t.Parallel()

	t.Run("simple", func(t *testing.T) {
		t.Parallel()

		dsn, err := url.Parse("memcached://host")
		require.NoError(t, err)
		client, err := New(dsn, nil)
		require.NoError(t, err)

		s, ok := client.(*store)
		require.True(t, ok)
		assert.EqualValues(t, 0, s.expire)
		assert.Equal(t, "texd/", s.keyPrefix)

		c, ok := s.client.(*memcache.Client)
		require.True(t, ok)
		assert.Equal(t, DefaultTimeout, c.Timeout)
		assert.Equal(t, DefaultMaxIdleConns, c.MaxIdleConns)
	})

	t.Run("allzezings", func(t *testing.T) {
		t.Parallel()

		dsn, err := url.Parse("memcached://host?expiration=10&timeout=2&max_idle_conns=1&key_prefix=prefix")
		require.NoError(t, err)
		client, err := New(dsn, nil)
		require.NoError(t, err)

		s, ok := client.(*store)
		require.True(t, ok)
		assert.Equal(t, 10*time.Second, s.expire)
		assert.Equal(t, "prefix", s.keyPrefix)

		c, ok := s.client.(*memcache.Client)
		require.True(t, ok)
		assert.Equal(t, 2*time.Second, c.Timeout)
		assert.Equal(t, 1, c.MaxIdleConns)
	})
}

func TestNew_invalidConfig(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ dsn, err string }{
		{
			dsn: "memcached://",
			err: "memcached: no server(s) configured",
		}, {
			dsn: "memcached://host?expiration=x",
			err: `memcached: invalid expiration parameter: time: invalid duration "x"`,
		}, {
			dsn: "memcached://host?timeout=x",
			err: `memcached: invalid timeout parameter: time: invalid duration "x"`,
		}, {
			dsn: "memcached://host?timeout=-1s",
			err: "memcached: negative timeout parameter",
		}, {
			dsn: "memcached://host?timeout=-1",
			err: "memcached: negative timeout parameter",
		}, {
			dsn: "memcached://host?max_idle_conns=x",
			err: `memcached: invalid max_idle_conns parameter: strconv.Atoi: parsing "x": invalid syntax`,
		}, {
			dsn: "memcached://host?max_idle_conns=-1",
			err: "memcached: negative max_idle_conns parameter",
		}, {
			dsn: "memcached://host?expiration=2562047h",
			err: "memcached: expiration parameter too large, must be <= 596523h14m7s",
		}, {
			dsn: "memcached://host?key_prefix=" + strings.Repeat("x", 210),
			err: "memcached: key_prefix parameter must be <= 207 characters",
		},
	} {
		dsn, err := url.Parse(tc.dsn)
		require.NoError(t, err, "dns: %s", tc.dsn)

		client, err := New(dsn, nil)
		require.EqualError(t, err, tc.err, "dns: %s", tc.dsn)
		require.Nil(t, client, "dns: %s", tc.dsn)
	}
}

type clientMock struct {
	mock.Mock
}

func (m *clientMock) Get(key string) (*memcache.Item, error) {
	args := m.Called(key)
	item := args.Get(0)
	if item == nil {
		return nil, args.Error(1)
	}
	return item.(*memcache.Item), args.Error(1)
}

func (m *clientMock) Set(item *memcache.Item) error {
	args := m.Called(item)
	return args.Error(0)
}

func (m *clientMock) Touch(key string, seconds int32) error {
	args := m.Called(key, seconds)
	return args.Error(0)
}

type storeSuite struct {
	suite.Suite

	store  *store
	client *clientMock
}

func TestStore(t *testing.T) {
	suite.Run(t, new(storeSuite))
}

func (s *storeSuite) SetupTest() {
	// the actual value does not matter
	cannedTime := time.Date(2022, time.January, 31, 18, 54, 43, 0, time.UTC)

	s.client = &clientMock{}
	s.store = &store{
		client:    s.client,
		expire:    DefaultExpiration,
		keyPrefix: DefaultKeyPrefix,
		clock:     internal.MockClock(cannedTime),
	}
}

func (s *storeSuite) TestExists() {
	s.client.On("Get", "texd/present").Return(&memcache.Item{}, nil)
	s.client.On("Get", "texd/absent").Return(nil, memcache.ErrCacheMiss)

	s.Assert().True(s.store.Exists("present"))
	s.Assert().False(s.store.Exists("absent"))
}

func (s *storeSuite) TestCopyFile() {
	s.client.On("Get", "texd/id").Return(&memcache.Item{
		Value: []byte("contents"),
	}, nil)

	var buf bytes.Buffer
	err := s.store.CopyFile(nil, "id", &buf)
	s.Require().NoError(err)
	s.Assert().Equal("contents", buf.String())
}

func (s *storeSuite) TestCopyFile_unknownRef() {
	s.client.On("Get", "texd/id").Return(nil, memcache.ErrCacheMiss)

	err := s.store.CopyFile(nil, "id", nil)
	s.Require().ErrorIs(err, refstore.ErrUnknownReference)
}

type failIO struct{}

func (failIO) Read([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (failIO) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (s *storeSuite) TestCopyFile_writeFailure() {
	s.client.On("Get", "texd/id").Return(&memcache.Item{
		Value: []byte("contents"),
	}, nil)

	var buf failIO
	err := s.store.CopyFile(nil, "id", &buf)
	s.Require().EqualError(err, "memcached: failed to copy storage object: io: read/write on closed pipe")
}

func (s *storeSuite) TestCopyFile_serverError() {
	s.client.On("Get", "texd/id").Return(nil, memcache.ErrServerError)

	err := s.store.CopyFile(nil, "id", nil)
	s.Require().ErrorIs(err, memcache.ErrServerError)
}

func (s *storeSuite) TestCopyFile_withRelativeExpiration() {
	exp := relativeExpirationThreshold - 1
	s.store.expire = exp

	s.client.On("Get", "texd/id").Return(&memcache.Item{
		Value: []byte("contents"),
	}, nil)

	s.client.On("Touch", "texd/id", int32(exp.Seconds())).Return(nil)

	var buf bytes.Buffer
	err := s.store.CopyFile(nil, "id", &buf)
	s.Require().NoError(err)
	s.Assert().Equal("contents", buf.String())
}

func (s *storeSuite) TestCopyFile_withAbsoluteExpiration() {
	exp := relativeExpirationThreshold
	s.store.expire = exp

	s.client.On("Get", "texd/id").Return(&memcache.Item{
		Value: []byte("contents"),
	}, nil)

	touch := int32(s.store.clock.Now().Add(exp).Unix())
	s.client.On("Touch", "texd/id", touch).Return(nil)

	var buf bytes.Buffer
	err := s.store.CopyFile(nil, "id", &buf)
	s.Require().NoError(err)
	s.Assert().Equal("contents", buf.String())
}

func (s *storeSuite) TestStore() {
	buf := bytes.NewBufferString("contents")
	id := refstore.NewIdentifier(buf.Bytes())

	s.client.On("Set", &memcache.Item{
		Key:        "texd/" + id.Raw(),
		Value:      buf.Bytes(),
		Expiration: 0,
	}).Return(nil)

	err := s.store.Store(nil, buf)
	s.Require().NoError(err)
}

func (s *storeSuite) TestStore_readError() {
	var r failIO
	err := s.store.Store(nil, &r)
	s.Require().EqualError(err, "memcached: failed to read reference file: io: read/write on closed pipe")
}

func (s *storeSuite) TestStore_serverError() {
	buf := bytes.NewBufferString("contents")
	id := refstore.NewIdentifier(buf.Bytes())

	s.client.On("Set", &memcache.Item{
		Key:        "texd/" + id.Raw(),
		Value:      buf.Bytes(),
		Expiration: 0,
	}).Return(memcache.ErrServerError)

	err := s.store.Store(nil, buf)
	s.Require().ErrorIs(err, memcache.ErrServerError)
}

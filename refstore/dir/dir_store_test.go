package dir

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"testing"

	"github.com/digineo/texd/refstore"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type dummyFile []byte

func (d dummyFile) ID() refstore.Identifier {
	return refstore.NewIdentifier(d)
}

func (d dummyFile) r() io.Reader {
	return bytes.NewReader(d)
}

func swapDefaultFs(t *testing.T, callback func()) {
	t.Helper()

	defaultFs = afero.NewMemMapFs()
	t.Cleanup(func() { defaultFs = afero.OsFs{} })

	callback()
}

func TestNew(t *testing.T) {
	dsn, err := url.Parse("dir:///")
	require.NoError(t, err)

	swapDefaultFs(t, func() {
		err := defaultFs.Chmod("/", 0o777)
		require.NoError(t, err)

		adapter, err := New(dsn, &refstore.KeepForever{})
		require.NoError(t, err)

		_, ok := adapter.(*dir)
		require.True(t, ok)
	})
}

func TestNew_primed(t *testing.T) {
	dsn, err := url.Parse("dir:///refs")
	require.NoError(t, err)

	swapDefaultFs(t, func() {
		err := defaultFs.Mkdir("/refs", 0o777)
		require.NoError(t, err)

		contents := []byte("blob")
		ref := refstore.NewIdentifier(contents)
		name := "/refs/" + ref.Raw()
		err = afero.WriteFile(defaultFs, name, contents, 0o644)
		require.NoError(t, err)

		_, err = New(dsn, &refstore.PurgeOnStart{})
		require.NoError(t, err)

		_, err = defaultFs.Stat(name)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestNew_dirNotWritable(t *testing.T) {
	dsn, err := url.Parse("dir:///")
	require.NoError(t, err)

	swapDefaultFs(t, func() {
		adapter, err := New(dsn, &refstore.KeepForever{})
		require.EqualError(t, err, `path "/" not writable: permission denied`)
		assert.Nil(t, adapter)
	})
}

func TestDirAdapter_keepFiles(t *testing.T) {
	require := require.New(t)
	log := zap.NewNop()

	subject, err := NewMemory(nil, &refstore.KeepForever{})
	require.NoError(err)

	a := dummyFile("aaa")

	err = subject.Store(log, a.r())
	require.NoError(err)
	require.True(subject.Exists(a.ID()))

	var buf bytes.Buffer
	err = subject.CopyFile(log, a.ID(), &buf)
	require.NoError(err)
	require.EqualValues(a, buf.String())
}

func TestDirAdapter_purgeFiles(t *testing.T) {
	require := require.New(t)

	subject, err := NewMemory(nil, &refstore.PurgeOnStart{})
	require.NoError(err)

	a := dummyFile("aaa")

	fs := subject.(*dir).fs
	err = afero.WriteFile(fs, "/refs/"+a.ID().Raw(), a, 0o600)
	require.NoError(err)

	err = subject.(*dir).initRetentionPolicy()
	require.NoError(err)

	require.False(subject.Exists(a.ID()))
}

func TestDirAdapter_accessMap(t *testing.T) {
	require := require.New(t)
	log := zap.NewNop()
	f := dummyFile("01234567890")

	for _, q := range []struct{ n, sz int }{
		{1, 0},
		{0, 10},
	} {
		rp, err := refstore.NewAccessList(q.n, q.sz)
		require.NoError(err)

		subject, err := NewMemory(nil, rp)
		require.NoError(err)

		err = subject.Store(log, f.r())
		require.NoError(err)
		require.True(subject.Exists(f.ID()))

		e := subject.(*dir).rp.Peek(f.ID())
		require.NotNil(e)
		require.Equal(f.ID(), e.ID)
		require.Equal(len(f), e.Size)
	}
}

func TestPathFromURL(t *testing.T) {
	for _, tc := range []struct{ url, path string }{
		{"dir://rel", ""},                // host is ignores; no path
		{"dir://./foo", "foo"},           // host is ignored
		{"dir://.", "."},                 // special case where host is not ignored
		{"dir:///abs/path", "/abs/path"}, // full qualified absolute path
		{"dir:///", "/"},                 // absolute path (unlikely to be used in production)
	} {
		u, err := url.Parse(tc.url)
		require.NoError(t, err, "url: %s", tc.url)
		assert.Equal(t, tc.path, pathFromURL(u), "url: %s", tc.url)
	}
}

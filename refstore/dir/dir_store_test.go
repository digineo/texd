package dir

import (
	"bytes"
	"io"
	"testing"

	"github.com/digineo/texd/refstore"
	"github.com/spf13/afero"
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

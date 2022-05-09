package dir

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/digineo/texd/refstore"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
)

type mockPolicy struct {
	primedWith []*refstore.FileRef
}

var _ refstore.RetentionPolicy = (*mockPolicy)(nil)

func (m *mockPolicy) Prime(refs []*refstore.FileRef) (evicted []*refstore.FileRef) {
	m.primedWith = refs
	return nil
}

func (m *mockPolicy) Touch(id refstore.Identifier)                          {}
func (m *mockPolicy) Add(*refstore.FileRef) (evicted []*refstore.FileRef)   { return }
func (m *mockPolicy) Peek(refstore.Identifier) (existing *refstore.FileRef) { return }

func TestInitRetentionPolicy(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(retentionInitializerSuite))
}

type retentionInitializerSuite struct {
	suite.Suite

	subject *dir
	fs      afero.Fs
	rp      *mockPolicy
}

func (s *retentionInitializerSuite) SetupTest() {
	s.fs = afero.NewMemMapFs()
	s.rp = &mockPolicy{}
	s.subject = &dir{s.fs, "/", s.rp}
}

func (s *retentionInitializerSuite) TestEmptyFs() {
	require := s.Require()

	require.NoError(s.subject.initRetentionPolicy())
	require.NotNil(s.rp.primedWith)
	require.Empty(s.rp.primedWith)
}

func (s *retentionInitializerSuite) TestWithFiles() {
	assert, require := s.Assert(), s.Require()

	// adding a few files
	dummies := []dummyFile{
		dummyFile("a"),
		dummyFile("bb"),
	}

	t0 := time.Now().Truncate(time.Hour)

	for i, df := range dummies {
		path := s.subject.idPath(df.ID())

		w, err := s.fs.OpenFile(path, os.O_CREATE, 0600)
		require.NoError(err)

		n, err := io.Copy(w, df.r())
		w.Close()
		require.NoError(err)
		require.EqualValues(len([]byte(df)), n)

		mtime := t0.Add(-time.Duration(i) * time.Minute)
		require.NoError(s.fs.Chtimes(path, time.Time{}, mtime))
	}

	require.NoError(s.subject.initRetentionPolicy())
	require.Len(s.rp.primedWith, 2)

	// order is reversed, dummies[0] is older than dummies[1]
	assert.Equal(s.rp.primedWith[0].ID, dummies[1].ID())
	assert.Equal(s.rp.primedWith[1].ID, dummies[0].ID())
}

func (s *retentionInitializerSuite) TestWithNonFileRefFiles() {
	require := s.Require()

	path := "/not-a-fileref"
	w, err := s.fs.OpenFile(path, os.O_CREATE, 0600)
	require.NoError(err)
	w.Close()

	errMsg := fmt.Sprintf("file %s does not look like a reference file: invalid identifier: unexpected input length", path)
	require.EqualError(s.subject.initRetentionPolicy(), errMsg)
}

func (s *retentionInitializerSuite) TestIgnoresDirectories() {
	require := s.Require()

	require.NoError(s.fs.Mkdir("/dir", 0700))
	w, err := s.fs.OpenFile("/dir/file", os.O_CREATE, 0600)
	require.NoError(err)
	w.Close()

	require.NoError(s.subject.initRetentionPolicy())
	require.Empty(s.rp.primedWith)
}

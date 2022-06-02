package tex

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFile_flags(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	subject := File{"", 0, 0}
	assert.False(subject.isCandidate())
	assert.False(subject.hasDocumentClass())
	assert.False(subject.hasTexdMark())

	subject.flags = flagCandidate
	assert.True(subject.isCandidate())
	assert.False(subject.hasDocumentClass())
	assert.False(subject.hasTexdMark())

	subject.flags = flagCandidate | flagTexdMark
	assert.True(subject.isCandidate())
	assert.False(subject.hasDocumentClass())
	assert.True(subject.hasTexdMark())

	// flagTexdMark XOR flagDocumentClass in practice
	subject.flags = flagCandidate | flagDocumentClass
	assert.True(subject.isCandidate())
	assert.True(subject.hasDocumentClass())
	assert.False(subject.hasTexdMark())
}

type nopCloser struct{ io.Writer }

func (*nopCloser) Close() error { return nil }

func testFileWriter(t *testing.T, s string, candidate bool) {
	t.Helper()

	var buf bytes.Buffer
	f := &File{}
	if candidate {
		f.flags = flagCandidate
	}

	subject := fileWriter{
		log:  zap.NewNop(),
		file: f,
		wc:   &nopCloser{Writer: &buf},
		buf:  make([]byte, 4),
	}

	n, err := subject.Write([]byte(s))
	require.NoError(t, err)
	assert.Equal(t, len(s), n)
	assert.EqualValues(t, s, buf.String())

	if candidate {
		offset := len(s)
		if max := len(subject.buf); offset > max {
			offset = max
		}
		assert.Equal(t, offset, subject.off)
	} else {
		assert.Equal(t, 0, subject.off)
	}
}

func TestFileWriter_nonCandicate(t *testing.T) {
	t.Parallel()
	testFileWriter(t, "Hello World!", false)
}

func TestFileWriter_candicateSmall(t *testing.T) {
	t.Parallel()
	testFileWriter(t, "Hi!", true)
}

func TestFileWriter_candicateLarge(t *testing.T) {
	t.Parallel()
	testFileWriter(t, strings.Repeat("a", 100), true)
}

func TestCleanpath(t *testing.T) {
	t.Parallel()

	t.Run("cleanable", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		for name, expected := range map[string]string{
			"aa.tex":      "aa.tex",
			"./bb.tex":    "bb.tex",
			"a/../cc.tex": "cc.tex",
			"dd.tex/.":    "dd.tex",
			"image.png":   "image.png",
		} {
			actual, ok := cleanpath(name)
			assert.True(ok, "expected %q to be cleanable", name)
			assert.Equal(expected, actual, "expected cleanpath(%q) to be %q, got %q",
				name, expected, actual)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		for _, name := range []string{
			// guard against path traversal
			"",
			".",
			"../a.tex",
			"../b/a.tex",
			"../../b.tex",
			"x/../../../../../../etc/passwd",
			"/etc/passwd",

			// guard against special chars
			"_underscore.tex",
			"&ampersand.tex",
			`\backslash.tex`,
			"`backtick.tex",
			`%percent.tex`,
			"$dollar.tex",
			"^caret.tex",
		} {
			actual, ok := cleanpath(name)
			assert.False(ok, "expected %q not to be cleanable (result was %q)",
				name, actual)
		}
	})
}

func TestIsMainCandidate(t *testing.T) {
	t.Parallel()

	t.Run("main candidates", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		for _, name := range []string{
			"input.tex", "main.tex", "document.tex", "foo.tex",
			"00-intro.tex",
			// XXX: We don't double check for file names with special chars
			// iu isMainCandidate(). They should be filtered out before even
			// reaching this point (see cleanpath).
			"zz_outro.tex", "ca$h.tex",
		} {
			is := isMainCandidate(name)
			assert.True(is, "expected %q to be a main candidate", name)
		}
	})

	t.Run("no candidates", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		for _, name := range []string{
			"_input.tex", "chapter/input.tex", "input.png", ".tex",
			"", "input.tex/", "/input.tex",
		} {
			is := isMainCandidate(name)
			assert.False(is, "expected %q not to be a main candidate", name)
		}
	})
}

type documentHelper struct {
	t  *testing.T
	fs afero.Afero
	*document
}

func (h *documentHelper) join(el ...string) string {
	return path.Join(h.workdir, path.Join(el...))
}

func (h *documentHelper) exists(el ...string) bool {
	h.t.Helper()
	ok, err := h.fs.Exists(h.join(el...))
	require.NoError(h.t, err)
	return ok
}

func (h *documentHelper) isDir(el ...string) bool {
	h.t.Helper()
	ok, err := h.fs.IsDir(h.join(el...))
	require.NoError(h.t, err)
	return ok
}

func (h *documentHelper) addFile(name, content string, flags candidateFlags) {
	h.t.Helper()

	require.False(h.t, h.exists(name))
	require.NoError(h.t, h.AddFile(name, content))
	require.True(h.t, h.exists(name))
	require.EqualValues(h.t, h.files[name], &File{
		name:  name,
		flags: flags,
		size:  len(content),
	})
}

func TestDocument(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// create a document and wrap in a test helper
	subject := documentHelper{ //nolint:forcetypeassert
		t:        t,
		fs:       afero.Afero{Fs: afero.NewMemMapFs()},
		document: NewDocument(zap.NewNop(), DefaultEngine, "").(*document),
	}
	subject.document.fs = subject.fs

	// grab working directory
	wd, err := subject.WorkingDirectory()
	require.NoError(err)
	ok, _ := subject.fs.Exists(wd)
	require.True(ok)

	// add some files
	subject.addFile("foo.tex", `\documentclass{article}`, flagCandidate|flagDocumentClass)
	subject.addFile("toc.tex", `\tableofcontents`, flagCandidate)

	main, err := subject.MainInput()
	require.NoError(err)
	require.Equal("foo.tex", main)

	// add a file with mark (which skips hasClass check)
	subject.addFile("main.tex", "%!texd\n\\documentclass{book}", flagCandidate|flagTexdMark)

	main, err = subject.MainInput()
	require.NoError(err)
	require.Equal("main.tex", main)

	// add file in subdirectory
	require.False(subject.exists("chapter"))
	subject.addFile("chapter/bar.tex", `\chapter{A Bar Runs Into A Priest}`, 0)
	require.True(subject.isDir("chapter"))
	require.Len(subject.files, 4)

	// try adding an invalid file
	err = subject.AddFile("../O_o.tex", "")
	catErr := &ErrWithCategory{}
	require.ErrorAs(err, &catErr)
	require.EqualValues(catErr, InputError("invalid file name", nil, map[string]interface{}{
		"filename": "../O_o.tex",
	}))
	require.Len(subject.files, 4) // no change

	// remove all files
	require.NoError(subject.Cleanup())
	require.False(subject.exists("chapter/bar.tex"))
	require.False(subject.exists("chapter"))
	require.False(subject.exists("foo.tex"))
}

func TestDocument_MainInput(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	subject := documentHelper{ //nolint:forcetypeassert
		t:        t,
		fs:       afero.Afero{Fs: afero.NewMemMapFs()},
		document: NewDocument(zap.NewNop(), DefaultEngine, "").(*document),
	}
	subject.document.fs = subject.fs

	_, err := subject.MainInput()
	assert.EqualError(err, "cannot determine main input file: no candidates")

	{ // adding files to root directory
		require.NoError(subject.AddFile("a.tex", ""))
		main, err := subject.MainInput()
		assert.NoError(err)
		assert.Equal("a.tex", main)

		// not a main candidate
		require.NoError(subject.AddFile("a/a.tex", ""))
		main, err = subject.MainInput()
		assert.NoError(err)
		assert.Equal("a.tex", main)

		// another candidate makes decision impossible
		require.NoError(subject.AddFile("b.tex", ""))
		_, err = subject.MainInput()
		assert.EqualError(err, "cannot determine main input file: multiple candidates")
	}

	{ // adding files with \documentclass overrides normal candidates
		require.NoError(subject.AddFile("c.tex", `\documentclass{book}`))
		main, err := subject.MainInput()
		assert.NoError(err)
		assert.Equal("c.tex", main)

		// another candidate makes decision impossible
		require.NoError(subject.AddFile("d.tex", `\documentclass{book}`))
		_, err = subject.MainInput()
		assert.EqualError(err, "cannot determine main input file: multiple files with \\documentclass")
	}

	{ // adding files with %!texd mark overrides \\documentclass candidates
		require.NoError(subject.AddFile("e.tex", Mark))
		main, err := subject.MainInput()
		assert.NoError(err)
		assert.Equal("e.tex", main)

		// another candidate makes decision impossible
		require.NoError(subject.AddFile("f.tex", Mark))
		_, err = subject.MainInput()
		assert.EqualError(err, "cannot determine main input file: multiple files with %!texd mark")
	}
}

func TestNewDocument(t *testing.T) {
	t.Parallel()

	engine := NewEngine("foo")
	image := "bar"

	subject := NewDocument(zap.NewNop(), engine, image)
	require.NotNil(t, subject)

	assert.Equal(t, engine, subject.Engine())
	assert.Equal(t, image, subject.Image())
}

type mockFs struct {
	mock.Mock
	afero.Fs
}

func (fs *mockFs) Mkdir(name string, mode fs.FileMode) error {
	args := fs.Called(name, mode)
	return args.Error(0)
}

func TestDocument_WorkingDirectory(t *testing.T) {
	t.Parallel()

	mockfs := &mockFs{}
	mockfs.On("Mkdir", mock.AnythingOfType("string"), fs.FileMode(0o700)).
		Return(os.ErrPermission).
		Times(1)

	subject := document{
		fs:        mockfs,
		mkWorkDir: &sync.Once{},
	}

	wd, err := subject.WorkingDirectory()
	require.EqualError(t, err, "creating working directory failed: permission denied")
	assert.Equal(t, wd, "")

	// mkWorkDir is not called again, otherwise we'll see a
	// "mock: The method has been called over 1 times" failure
	_, err = subject.WorkingDirectory()
	require.Error(t, err)
}

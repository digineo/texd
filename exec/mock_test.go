package exec

import (
	"io"
	"testing"

	"github.com/digineo/texd/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMock(t *testing.T) {
	doc := &mockDocument{"/", nil, "a", nil}
	subject := Mock(true, "content")(doc)

	mock, ok := subject.(*MockExec)
	require.True(t, ok)
	assert.True(t, mock.ShouldFail)
	assert.Equal(t, "content", mock.ResultContents)
	assert.Equal(t, doc, mock.baseExec.doc)
}

func TestMock_Run_extractError(t *testing.T) {
	doc := &mockDocument{"/", io.ErrClosedPipe, "a", nil}
	subject := Mock(true, "content")(doc)

	err := subject.Run(bg, xlog.NewNop())
	require.EqualError(t, err, "invalid document: "+io.ErrClosedPipe.Error())
}

func TestMock_Run_invalidMainfilePanics(t *testing.T) {
	doc := &mockDocument{"/", nil, "a", nil} // doc.main is malformed
	subject := Mock(true, "content")(doc)

	require.PanicsWithValue(t, "malformed input file: missing extension",
		func() { _ = subject.Run(bg, xlog.NewNop()) })
}

func TestMock_Run_noAddFilePanics(t *testing.T) {
	doc := &mockDocument{"/", nil, "a.tex", nil} // doesn't implement AddFile
	subject := Mock(true, "content")(doc)

	require.PanicsWithValue(t, "can't add files to document",
		func() { _ = subject.Run(bg, xlog.NewNop()) })
}

type mockDockumentWithAddFile struct {
	mock.Mock
	*mockDocument
}

func (m *mockDockumentWithAddFile) AddFile(name, contents string) error {
	args := m.Called(name, contents)
	return args.Error(0)
}

func TestMock_Run_errorOnAddFilePanics(t *testing.T) {
	doc := &mockDockumentWithAddFile{
		mockDocument: &mockDocument{"/", nil, "a.tex", nil},
	}
	subject := Mock(true, "content")(doc)

	doc.On("AddFile", "a.log", "content").Return(io.ErrClosedPipe)

	require.PanicsWithError(t, "failed to store result file: "+io.ErrClosedPipe.Error(),
		func() { _ = subject.Run(bg, xlog.NewNop()) })
}

func TestMock_Run_shouldFailCapturesLog(t *testing.T) {
	doc := &mockDockumentWithAddFile{
		mockDocument: &mockDocument{"/", nil, "a.tex", nil},
	}
	subject := Mock(true, "content")(doc)

	doc.On("AddFile", "a.log", "content").Return(nil)

	err := subject.Run(bg, xlog.NewNop())
	require.EqualError(t, err, "compilation failed")
}

func TestMock_Run_shouldFailCapturesResult(t *testing.T) {
	doc := &mockDockumentWithAddFile{
		mockDocument: &mockDocument{"/", nil, "a.tex", nil},
	}
	subject := Mock(false, "%PDF/1.5")(doc)

	doc.On("AddFile", "a.pdf", "%PDF/1.5").Return(nil)

	err := subject.Run(bg, xlog.NewNop())
	require.NoError(t, err)
}

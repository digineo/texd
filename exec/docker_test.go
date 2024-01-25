package exec

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/digineo/texd/tex"
	"github.com/digineo/texd/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type dockerClientMock struct {
	mock.Mock
}

func (m *dockerClientMock) Run(ctx context.Context, tag, wd string, cmd []string) (string, error) {
	args := m.Called(ctx, tag, wd, cmd)
	return args.String(0), args.Error(1)
}

func TestDockerClient_Executor(t *testing.T) {
	subject := (&DockerClient{
		log: xlog.NewNop(),
		cli: &apiMock{},
	}).Executor(&mockDocument{})
	require.NotNil(t, subject)
}

func TestDockerExec_invalidDocument(t *testing.T) {
	doc := &mockDocument{"", io.ErrClosedPipe, "b", nil}
	exec := &dockerExec{
		baseExec: baseExec{doc: doc},
		cli:      nil, // not accessed
	}

	err := exec.Run(bg, xlog.NewNop())
	require.EqualError(t, err, "invalid document: "+io.ErrClosedPipe.Error())
}

func TestDockerExec_latexmkFailed(t *testing.T) {
	mainFile := "index.tex"
	doc := &mockDocument{"/texd", nil, mainFile, nil}
	errStart := errors.New("command not found")

	cli := &dockerClientMock{}
	cli.On("Run", bg, "", "/texd", doc.Engine().LatexmkCmd(mainFile)).
		Return("outputlog", errStart)

	exec := &dockerExec{
		baseExec: baseExec{doc: doc},
		cli:      cli,
	}

	err := exec.Run(bg, xlog.NewNop())
	require.EqualError(t, err, "compilation failed: "+errStart.Error())
	assert.True(t, tex.IsCompilationError(err))

	catErr, ok := err.(*tex.ErrWithCategory)
	require.True(t, ok)

	kv := catErr.Extra()
	assert.EqualValues(t, kv["output"], "outputlog")
}

func TestDockerExec_success(t *testing.T) {
	mainFile := "index.tex"
	doc := &mockDocument{"/texd", nil, mainFile, nil}
	cli := &dockerClientMock{}
	cli.On("Run", bg, "", "/texd", doc.Engine().LatexmkCmd(mainFile)).
		Return("", nil)

	exec := &dockerExec{
		baseExec: baseExec{doc: doc},
		cli:      cli,
	}

	err := exec.Run(bg, xlog.NewNop())
	require.NoError(t, err)
}

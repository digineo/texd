package exec

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/digineo/texd/tex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLocalExec_Run(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// create tempdir
	tmpDir, err := os.MkdirTemp("/tmp", "texd")
	require.NoError(err)
	defer os.RemoveAll(tmpDir)

	// fill tempdir
	err = os.WriteFile(path.Join(tmpDir, "main.tex"), []byte("hello world"), 0o600)
	require.NoError(err)

	latexmkPath, err := filepath.Abs("testdata/latexmk")
	require.NoError(err)

	tests := []struct {
		doc            Document
		path           string
		expectedErr    string
		expectedOutput string
	}{
		{
			doc:  &mockDocument{tmpDir, nil, "main.tex", nil},
			path: "/bin/true",
		},
		{
			doc:         &mockDocument{"", io.ErrClosedPipe, "main.tex", nil},
			path:        "/bin/false",
			expectedErr: "invalid document: io: read/write on closed pipe",
		},
		{
			doc:            &mockDocument{tmpDir, nil, "main.tex", nil},
			path:           latexmkPath,
			expectedErr:    "compilation failed: exit status 23",
			expectedOutput: tmpDir + " -cd -silent -pv- -pvc- -pdfxe main.tex\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			// create local exec
			exec := LocalExec(tt.doc).(*localExec)
			exec.path = tt.path
			err := exec.Run(context.Background(), zap.NewNop())

			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else if assert.EqualError(t, err, tt.expectedErr) {
				cErr := err.(*tex.ErrWithCategory)
				if tt.expectedOutput != "" {
					assert.Equal(t, tt.expectedOutput, cErr.Extra()["output"])
				}
			}
		})
	}

	require.NoError(err)
}

package exec

import (
	"context"
	"io/ioutil"
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
	logger, err := zap.NewDevelopment()
	require.NoError(err)

	// create tempdir
	tmpDir, err := ioutil.TempDir("/tmp", "texd")
	require.NoError(err)
	defer os.RemoveAll(tmpDir)

	// fill tempdir
	err = os.WriteFile(path.Join(tmpDir, "main.tex"), []byte("hello world"), 0o600)
	require.NoError(err)

	doc := mockDocument{tmpDir, nil, "main.tex", nil}
	ctx := context.Background()

	latexmkPath, err := filepath.Abs("testdata/latexmk")
	require.NoError(err)

	tests := []struct {
		path           string
		expectedErr    string
		expectedOutput string
	}{
		{
			path: "/bin/true",
		},
		{
			path:           latexmkPath,
			expectedErr:    "compilation failed: exit status 23",
			expectedOutput: tmpDir + " -cd -silent -pv- -pvc- -pdfxe main.tex\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			// create local exec
			exec := LocalExec(&doc).(*localExec)
			exec.path = tt.path
			err := exec.Run(ctx, logger)

			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else if assert.EqualError(t, err, tt.expectedErr) {
				cErr := err.(*tex.ErrWithCategory)
				assert.Equal(t, tt.expectedOutput, cErr.Extra()["output"])
			}
		})
	}

	require.NoError(err)
}

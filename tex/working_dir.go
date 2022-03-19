package tex

import (
	"fmt"
	"path"

	"github.com/digineo/texd/internal"
)

// baseJobDir is the directory in which texd will create its job
// sub-directories. It follows the semantic of os.CreateTemp: when
// blank, users shall fall back to os.TempDir.
var baseJobDir string

type ErrInvalidWorkDir struct {
	dir   string
	cause error
}

func (err *ErrInvalidWorkDir) Error() string {
	if err.cause == nil {
		return fmt.Sprintf("invalid working directory %q", err.dir)
	}
	return fmt.Sprintf("invalid working directory %q: %v", err.dir, err.cause)
}

func (err *ErrInvalidWorkDir) Unwrap() error {
	return err.cause
}

// SetJobBaseDir will update the working directory for texd.
// If dir is empty, texd will fallback to os.TempDir(). The directory
// must exist, and it must be writable, otherwise a non-nil error is
// returned.
func SetJobBaseDir(dir string) error {
	if dir != "" {
		dir = path.Clean(dir)
		if err := internal.EnsureWritable(osfs, dir); err != nil {
			return &ErrInvalidWorkDir{dir, err}
		}
	}

	baseJobDir = dir
	return nil
}

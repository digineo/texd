package tex

import (
	"fmt"
	"os"
	"syscall"
)

// baseJobDir is the directory in which texd will create its
// job sub-directories.
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
	if dir == "" {
		dir = os.TempDir()
	}

	st, err := osfs.Stat(dir)
	if err != nil {
		return &ErrInvalidWorkDir{dir, err}
	}
	if !st.IsDir() {
		return &ErrInvalidWorkDir{dir, os.ErrInvalid}
	}

	// check permissions
	var ok bool
	switch mod := st.Mode(); {
	case mod&0o002 != 0: // world writable
		ok = true
	case mod&0o020 != 0: // group writable
		ok = matchEGID(st, os.Getegid())
	case mod&0o200 != 0: // owner writable
		ok = matchEUID(st, os.Geteuid())
	}
	if !ok {
		return &ErrInvalidWorkDir{dir, os.ErrPermission}
	}

	baseJobDir = dir
	return nil
}

func stat_t(st os.FileInfo) *syscall.Stat_t {
	switch typ := st.Sys().(type) {
	case syscall.Stat_t:
		return &typ
	case *syscall.Stat_t:
		return typ
	default:
		return nil
	}
}

func matchEGID(st os.FileInfo, egid int) bool {
	sys := stat_t(st)
	return sys != nil && int(sys.Gid) == egid
}

func matchEUID(st os.FileInfo, euid int) bool {
	sys := stat_t(st)
	return sys != nil && int(sys.Uid) == euid
}

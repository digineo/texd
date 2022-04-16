package internal

import (
	"os"
	"path"
	"syscall"

	"github.com/spf13/afero"
)

// EnsureWritable checks whether the given directory is writable to the
// current user, and return an error, if it is not.
func EnsureWritable(fs afero.Fs, dir string) error {
	if !path.IsAbs(dir) {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = path.Join(wd, dir)
	}

	st, err := fs.Stat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return os.ErrInvalid
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
		return os.ErrPermission
	}
	return nil
}

func owner(st os.FileInfo) (uid, gid int, ok bool) {
	switch typ := st.Sys().(type) {
	case syscall.Stat_t:
		return int(typ.Uid), int(typ.Gid), true
	case *syscall.Stat_t:
		return int(typ.Uid), int(typ.Gid), true
	default:
		return 0, 0, false
	}
}

func matchEGID(st os.FileInfo, egid int) bool {
	_, gid, ok := owner(st)
	return ok && gid == egid
}

func matchEUID(st os.FileInfo, euid int) bool {
	uid, _, ok := owner(st)
	return ok && uid == euid
}

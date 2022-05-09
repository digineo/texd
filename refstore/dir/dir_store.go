// Package dir implements an on-disk reference storage adapter.
//
// To be able to use it, add an anonymous import to your main package:
//
//	import _ "github.com/digineo/texd/refstore/dir"
//
// This registers the "dir://" and "memory://" adapters.
//
// For configuration, use a DSN with the following shape:
//
//	dsn := "dir:///path/for/persistent/files?options"
//	dir, _ := refstore.NewStore(dsn, &refstore.KeepForever{})
//
// Note the triple-slash: the scheme is "dir://", and the directory
// itself is /path/for/persistent/files. See New() for valid options.
//
// Mostly for testing, this also provides a memory-backed adapter. It is
// STRONGLY recommended to use it only with a size-limited access list,
// as it may otherwise consume all available RAM:
//
//	rp, _ := refstore.NewAccessList(1000, 100 * units.MiB)
//	mem, _ := restore.NewStore("memory:", rp)
package dir

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/digineo/texd/internal"
	"github.com/digineo/texd/refstore"
	"github.com/spf13/afero"
	"go.uber.org/zap"
)

var defaultFs afero.Fs = afero.OsFs{} // swapped in tests

func init() {
	refstore.RegisterAdapter("dir", New)
	refstore.RegisterAdapter("memory", NewMemory)
}

type dir struct {
	fs   afero.Fs
	path string
	rp   refstore.RetentionPolicy
}

// New returns a new storage adapter.
//
// This adapter takes only the path from the config URL. The path must
// point to an existing directory, and it must be writable, otherwise
// New returns an error.
//
// The retention policy is applied immediately: in case of refstore.PurgeOnStart,
// existing file references will be deleted, and in case of refstore.AccessList,
// references are first ordered by mtime and added in order to the retention
// policies internal state (which may evict and delete files, if the policies
// limits are reached).
func New(config *url.URL, rp refstore.RetentionPolicy) (refstore.Adapter, error) {
	d := &dir{
		fs:   defaultFs,
		path: pathFromURL(config),
		rp:   rp,
	}
	if err := internal.EnsureWritable(d.fs, d.path); err != nil {
		return nil, fmt.Errorf("path %q not writable: %w", d.path, err)
	}
	if err := d.initRetentionPolicy(); err != nil {
		return nil, err
	}
	return d, nil
}

// NewMemory returns a memory-backed reference store adapter.
//
// The config URL is ignored, all reference files are stored in a virtual
// file system at "/".
//
// Like New, The retention policy is applied immediately. Note that due
// to the nature of (volatile) memory, refstore.PurgeOnStart and refstore.KeepForever
// behave the same way.
//
// It is STRONGLY recommended to use NewMemotry only with a size-limited
// access list, as it may otherwise consume all available RAM.
func NewMemory(config *url.URL, rp refstore.RetentionPolicy) (refstore.Adapter, error) {
	d := &dir{
		fs:   afero.NewMemMapFs(),
		path: "/",
		rp:   rp,
	}
	_ = d.fs.Mkdir(d.path, 0o755)
	return d, nil
}

func (d *dir) Exists(id refstore.Identifier) bool {
	_, err := d.fs.Stat(d.idPath(id))
	return err == nil
}

func pathFromURL(config *url.URL) string {
	path := config.Path
	if config.Host == "." {
		path = filepath.Join(".", path)
	}
	return path
}

func (d *dir) CopyFile(log *zap.Logger, id refstore.Identifier, dst io.Writer) error {
	src, err := d.fs.Open(d.idPath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return refstore.ErrUnknownReference
		}
		return fmt.Errorf("unexpected error when accessing storage object: %v", err)
	}
	defer src.Close()

	log.Debug("copy file",
		zap.String("refstore", "disk"),
		zap.String("id", id.Raw()))
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy storage object: %v", err)
	}
	d.rp.Touch(id)
	return nil
}

func (d *dir) Store(log *zap.Logger, r io.Reader) error {
	tmp, err := afero.TempFile(d.fs, d.path, "tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create storage object: %v", err)
	}
	defer tmp.Close()

	sz := sizeWriter{tmp, 0}
	tee := io.TeeReader(r, &sz)
	id, err := refstore.ReadIdentifier(tee)
	if err != nil {
		_ = d.fs.Remove(tmp.Name())
		return fmt.Errorf("failed to create storage object: %v", err)
	}

	log.Debug("store file",
		zap.String("refstore", "disk"),
		zap.String("id", id.Raw()))

	dst := d.idPath(id)
	if err = d.fs.Rename(tmp.Name(), dst); err != nil {
		_ = d.fs.Remove(tmp.Name())
		_ = d.fs.Remove(dst)
		return fmt.Errorf("failed to create storage object %s: %w", id.String(), err)
	}

	d.rp.Add(&refstore.FileRef{ID: id, Size: sz.n})
	return nil
}

func (d *dir) remove(id refstore.Identifier) error {
	if err := d.fs.Remove(d.idPath(id)); err != nil {
		return fmt.Errorf("failed to delete storage object %s: %w", id.String(), err)
	}
	return nil
}

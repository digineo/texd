// Package dir implements an on-disk reference storage adapter.
//
// To use it, add a anonymous import to your main package:
//
//	import (
//		_ "github.com/digineo/texd/refstore/dir"
//	)
//
// For configuration, use a DSN with the following shape:
//
//	dsn := "dir:///path/for/persistent/files?options"
//	dir, _ := refstore.NewStore(dsn)
//
// Note the triple-slash: the scheme is "dir://", and the directory
// itself is /path/for/persistent/files. See New() for valid options.
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

func init() {
	refstore.RegisterAdapter("dir", New)
}

type dir struct {
	fs   afero.Fs
	path string
}

// New returns a new storage adapter.
//
// The following options (config.Query()) are understood:
// - none yet
//
// The config.Path must point to an existing directory, and it must be
// writable.
func New(config *url.URL) (refstore.Adapter, error) {
	a := &dir{
		fs:   afero.OsFs{},
		path: configurePath(config),
	}
	if err := internal.EnsureWritable(a.fs, a.path); err != nil {
		return nil, fmt.Errorf("path %q not writable: %w", a.path, err)
	}
	return a, nil
}

func NewMemory(config *url.URL) (refstore.Adapter, error) {
	a := &dir{
		fs:   afero.NewMemMapFs(),
		path: configurePath(config),
	}
	_ = a.fs.MkdirAll(a.path, 0o755)
	return a, nil
}

func configurePath(config *url.URL) string {
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
	return nil
}

func (d *dir) Store(log *zap.Logger, contents []byte) error {
	id := refstore.NewIdentifier(contents)
	log.Debug("store file",
		zap.String("refstore", "disk"),
		zap.String("id", id.Raw()))

	if err := afero.WriteFile(d.fs, d.idPath(id), contents, 0o600); err != nil {
		return fmt.Errorf("failed to create storage object: %v", err)
	}
	return nil
}

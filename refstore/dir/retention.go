package dir

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"time"

	"github.com/digineo/texd/refstore"
	"github.com/spf13/afero"
)

// fileRef extends refstore.FileRef with a modification time.
type fileRef struct {
	mtime time.Time
	refstore.FileRef
}

func (d *dir) initRetentionPolicy() error {
	var files []*fileRef

	walkErr := afero.Walk(d.fs, d.path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != d.path {
				return fs.SkipDir // don't descent
			}
			return nil
		}

		name := info.Name()
		id, err := refstore.ParseIdentifier([]byte("sha256:" + name))
		if err != nil {
			// TODO: silently ignore?
			fullpath := filepath.Join(d.path, name)

			return fmt.Errorf("file %s does not look like a reference file: %w",
				fullpath, err)
		}

		files = append(files, &fileRef{
			mtime: info.ModTime(),
			FileRef: refstore.FileRef{
				ID:   id,
				Size: int(info.Size()),
			},
		})
		return nil
	})

	if walkErr != nil {
		return walkErr
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].mtime.Before(files[j].mtime)
	})

	prime := make([]*refstore.FileRef, 0, len(files))
	for _, f := range files {
		prime = append(prime, &f.FileRef)
	}

	for _, evicted := range d.rp.Prime(prime) {
		if err := d.remove(evicted.ID); err != nil {
			return err
		}
	}
	return nil
}

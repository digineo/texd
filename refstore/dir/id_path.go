package dir

import (
	"path/filepath"

	"github.com/digineo/texd/refstore"
)

func (d *dir) idPath(id refstore.Identifier) string {
	return filepath.Join(d.path, id.Raw())
}

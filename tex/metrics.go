package tex

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

type fileCategory byte

func categoryFromName(name string) fileCategory {
	name = filepath.Base(name)
	dot := strings.LastIndexByte(name, '.')
	if dot <= 0 || dot+1 == len(name) {
		// no dot, or dotfile, or dot at end of name
		return otherFile
	}

	for cat, exts := range fileCategories {
		for _, ext := range exts {
			if ext == name[dot+1:] {
				return cat
			}
		}
	}
	return otherFile
}

func (cat fileCategory) String() string {
	switch cat {
	case texFile:
		return "tex"
	case dataFile:
		return "data"
	case assetFile:
		return "asset"
	case otherFile:
		return "other"
	default:
		return fmt.Sprintf("%%!unknown(%#02x)", byte(cat))
	}
}

const (
	otherFile fileCategory = iota
	texFile
	assetFile
	dataFile
)

var fileCategories = map[fileCategory][]string{
	texFile: {"tex", "sty", "cls", "bib", "bbl", "lco"},
	assetFile: {
		"png", "jpg", "jpeg", "gif", // bitmap images
		"pdf", "eps", "svg", // vector images
		"ttf", "otf", "mf", "pfm", "pfb", // fonts
	},
	dataFile: {"csv", "xml", "json"},
}

// Metrics hold file sizes for input and output files. Each category
// field (TexFiles, AssetFiles, ...) is a slice with one size entry
// per file.
type Metrics struct {
	// TexFiles covers .tex, .sty, .cls and similar files.
	TexFiles []int
	// AssetFiles covers image files (.png, .jpg), font files (.ttf, .otf)
	// and other .pdf files.
	AssetFiles []int
	// DateFiles covers .csv, .xml and .json files.
	DataFiles []int
	// OtherFiles includes files not covered by other categories.
	OtherFiles []int
	// ResultFile covers the compiled PDF document. A value of -1
	// means that no PDF was produced.
	Result int
}

func (doc *document) Metrics() (m Metrics) {
	for name, f := range doc.files {
		switch cat := categoryFromName(name); cat {
		case texFile:
			m.TexFiles = append(m.TexFiles, f.size)
		case assetFile:
			m.AssetFiles = append(m.AssetFiles, f.size)
		case dataFile:
			m.DataFiles = append(m.DataFiles, f.size)
		default:
			m.OtherFiles = append(m.OtherFiles, f.size)
		}
	}

	m.Result = -1
	if input, err := doc.MainInput(); err == nil {
		if extpos := strings.LastIndexByte(input, '.'); extpos > 0 {
			path := path.Join(doc.workdir, input[:extpos]+".pdf")
			if s, err := doc.fs.Stat(path); err == nil {
				m.Result = int(s.Size())
			}
		}
	}

	return m
}

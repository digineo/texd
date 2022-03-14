package tex

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/spf13/afero"
)

// Mark is used to help identifying the main input file from a list
// of potential candidates. This is a last resort measurement, clients
// should specify main files explicitly.
const Mark = "%!texd"

// osfs can be overridden in tests.
var osfs = afero.NewOsFs()

// ForbiddenFiles is a list of file names which are not allowed for
// security reasons.
var ForbiddenFiles = []string{
	// latexmk config files are Perl scripts
	"latexmkrc", ".latexmkrc",
}

type candidateFlags uint8

const (
	flagCandidate candidateFlags = 1 << iota
	flagDocumentClass
	flagTexdMark
)

type File struct {
	name  string
	flags candidateFlags
}

func (f *File) isCandidate() bool      { return f.flags&flagCandidate > 0 }
func (f *File) hasDocumentClass() bool { return f.flags&flagDocumentClass > 0 }
func (f *File) hasTexdMark() bool      { return f.flags&flagTexdMark > 0 }

type Document interface {
	WorkingDirectory() (string, error)
	AddFile(name, contents string) error
	AddFiles(req *http.Request) error
	Cleanup() error
	Image() string
	Engine() Engine
	SetMainInput(string) error
	MainInput() (string, error)
	GetResult() (io.ReadCloser, error)
	GetLogs() (io.ReadCloser, error)
}

type document struct {
	fs afero.Fs // when nil, use osfs

	workdir   string
	files     map[string]*File
	mainInput string // only present after SetMainInput(), otherwise ask MainInput()

	image  string
	engine Engine

	mkWorkDir    *sync.Once
	mkWorkDirErr error
	*sync.RWMutex
}

var _ Document = (*document)(nil)

func NewDocument(engine Engine, image string) Document {
	return &document{
		fs:        osfs,
		files:     make(map[string]*File),
		image:     image,
		engine:    engine,
		mkWorkDir: &sync.Once{},
		RWMutex:   &sync.RWMutex{},
	}
}

func (doc *document) Image() string  { return doc.image }
func (doc *document) Engine() Engine { return doc.engine }

func (doc *document) WorkingDirectory() (string, error) {
	doc.mkWorkDir.Do(doc.createWorkDir)
	return doc.workdir, doc.mkWorkDirErr
}

func (doc *document) createWorkDir() {
	if wd, err := afero.TempDir(doc.fs, baseJobDir, "texd-"); err != nil {
		doc.mkWorkDirErr = UnknownError("creating working directory failed", err, nil)
	} else {
		doc.workdir = wd
	}
}

func (doc *document) AddFile(name, contents string) (err error) {
	file := &File{}

	defer func() {
		// add file name as context to error
		if err != nil {
			if cat, ok := err.(*ErrWithCategory); ok {
				cat.extra = KV{"filename": name}
			}
			// cleanup file list
			if file.name != "" {
				doc.Lock()
				delete(doc.files, file.name)
				doc.Unlock()
			}
		}
	}()

	var ok bool
	file.name, ok = cleanpath(name)
	if !ok {
		err = InputError("invalid file name", nil, nil)
		return
	}

	doc.Lock()
	if _, exists := doc.files[name]; exists {
		doc.Unlock()
		err = InputError("duplicate file name", nil, nil)
		return
	} else {
		doc.files[name] = file
		doc.Unlock()
	}

	var wd string
	wd, err = doc.WorkingDirectory()
	if err != nil {
		return // err is already an errWithCategory
	}

	if dir := path.Dir(name); dir != "" {
		if osErr := doc.fs.MkdirAll(path.Join(wd, dir), 0o700); osErr != nil {
			err = InputError("cannot create directory", osErr, nil)
			return
		}
	}

	f, osErr := doc.fs.OpenFile(path.Join(wd, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	defer func() { _ = f.Close() }()
	if osErr != nil {
		err = InputError("cannot create file", osErr, nil)
		return
	}

	_, osErr = f.Write([]byte(contents))
	if osErr != nil {
		err = InputError("cannot save file", osErr, nil)
		return
	}

	if isMainCandidate(file.name) {
		file.flags |= flagCandidate
		if strings.HasPrefix(contents, Mark) {
			file.flags |= flagTexdMark
		} else {
			max := len(contents)
			if max > 1024 {
				max = 1024
			}
			if strings.Contains(contents[:max], `\documentclass`) {
				file.flags |= flagDocumentClass
			}
		}
	}

	return nil
}

func (doc *document) AddFiles(req *http.Request) error {
	if mf := req.MultipartForm; mf != nil {
		defer func() { _ = mf.RemoveAll() }()

		var buf bytes.Buffer
		for name, files := range mf.File {
			for _, f := range files {
				rc, err := f.Open()
				if err != nil {
					return InputError("unable to open file", err, KV{"name": name})
				}
				defer rc.Close()

				buf.Reset()
				if _, err = io.Copy(&buf, rc); err != nil {
					return InputError("failed to read file", err, KV{"name": name})
				}

				if err := doc.AddFile(name, buf.String()); err != nil {
					return err
				}
			}
		}
	}

	for name, contents := range req.PostForm {
		// AddFile() will only accept one body per file name and construct
		// a proper error message. No need to perform something akin to
		// `AddFile(contents[0]) if len(contents) == 1` here.
		for _, content := range contents {
			if err := doc.AddFile(name, content); err != nil {
				return err
			}
		}
	}
	return nil
}

func (doc *document) SetMainInput(name string) error {
	doc.RLock()
	defer doc.RUnlock()

	if _, ok := doc.files[name]; ok {
		doc.mainInput = name
		return nil
	}

	return InputError("unknown input file name", nil, nil)
}

func (doc *document) MainInput() (string, error) {
	if doc.mainInput != "" {
		return doc.mainInput, nil
	}

	var withDocClass, withMark, others []*File

	doc.RLock()
	for _, f := range doc.files {
		if f.hasTexdMark() {
			withMark = append(withMark, f)
		} else if f.hasDocumentClass() {
			withDocClass = append(withDocClass, f)
		} else if f.isCandidate() {
			others = append(others, f)
		}
	}
	doc.RUnlock()

	for _, candidates := range []struct {
		files   []*File
		context string
	}{
		{withMark, "multiple files with " + Mark + " mark"},
		{withDocClass, "multiple files with \\documentclass"},
		{others, "multiple candidates"},
	} {
		if n := len(candidates.files); n == 1 {
			return candidates.files[0].name, nil
		} else if n > 1 {
			msg := "cannot determine main input file: " + candidates.context
			return "", InputError(msg, nil, KV{"candidates": candidates.files})
		}
	}

	return "", InputError("cannot determine main input file: no candidates", nil, nil)
}

// openFile opens an auxiliary file for reading. Auxiliary files are files
// with the same name stem as the main input file, but with a different
// extension.
func (doc *document) openFile(ext string) (io.ReadCloser, error) {
	input, err := doc.MainInput()
	if err != nil { // unlikely at this point
		return nil, InputError("no main input specified", err, nil)
	}

	extpos := strings.LastIndexByte(input, '.')
	if extpos <= 0 {
		return nil, InputError("invalid main input file name", nil, nil)
	}

	output := input[:extpos] + ext
	f, err := doc.fs.Open(path.Join(doc.workdir, output))
	if err != nil {
		return nil, CompilationError("failed to open output file for reading", err, KV{
			"file": output,
		})
	}
	return f, nil
}

func (doc *document) GetResult() (io.ReadCloser, error) {
	return doc.openFile(".pdf")
}

func (doc *document) GetLogs() (io.ReadCloser, error) {
	return doc.openFile(".log")
}

func cleanpath(name string) (clean string, ok bool) {
	clean = path.Clean(name)
	if /* current directory */ clean == "." ||
		/* forbidden file name */ isForbidden(name) ||
		/* directory traversal */ strings.HasPrefix(clean, "..") ||
		/* absolute paths */ strings.HasPrefix(clean, "/") ||
		/* easyly abusable TeX chars */ strings.ContainsAny(clean, "\\%$_^&`") {
		return "", false
	}
	ok = true
	return
}

func isForbidden(name string) bool {
	for _, n := range ForbiddenFiles {
		if n == name {
			return true
		}
	}
	return false
}

func isMainCandidate(name string) bool {
	if len(name) == 0 {
		return false
	}
	if strings.ContainsRune(name, os.PathSeparator) {
		return false
	}
	if !strings.HasSuffix(name, ".tex") {
		return false
	}
	if name == "input.tex" || name == "main.tex" || name == "document.tex" {
		return true
	}
	c := name[0]
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

func (doc *document) Cleanup() error {
	doc.Lock()
	defer doc.Unlock()

	if doc.workdir != "" {
		if err := doc.fs.RemoveAll(doc.workdir); err != nil {
			return InputError("cleanup failed", err, nil)
		}
		doc.workdir = ""
	}
	return nil
}

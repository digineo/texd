package tex

import (
	"bytes"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/spf13/afero"
	"go.uber.org/zap"
)

// Mark is used to help identifying the main input file from a list
// of potential candidates. This is a last resort measurement, clients
// should specify main files explicitly.
const Mark = "%!texd"

// texFs can be overridden in tests.
var texFs = afero.NewOsFs()

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
	size  int
}

func (f *File) isCandidate() bool      { return f.flags&flagCandidate > 0 }
func (f *File) hasDocumentClass() bool { return f.flags&flagDocumentClass > 0 }
func (f *File) hasTexdMark() bool      { return f.flags&flagTexdMark > 0 }

type fileWriter struct {
	log  *zap.Logger
	buf  []byte // the first few bytes written to wc
	off  int    // how many bytes were written to buf
	wc   io.WriteCloser
	file *File
}

func (w *fileWriter) Write(p []byte) (int, error) {
	if w.file.isCandidate() {
		// fill buf, if buf has capacity
		if max := len(w.buf); w.off < max {
			if n := max - w.off; n > len(p) {
				// p fits completely into buf's rest capacity
				copy(w.buf[w.off:], p)
				w.off += len(p)
			} else {
				// only the first n bytes fit into buf
				copy(w.buf[w.off:], p[:n])
				w.off += n
			}
		}
	}

	n, err := w.wc.Write(p)
	w.file.size += n
	return n, err
}

func (w *fileWriter) Close() error {
	if w.file.isCandidate() {
		if bytes.HasPrefix(w.buf, []byte(Mark)) {
			w.log.Info("found mark")
			w.file.flags |= flagTexdMark
		} else {
			if bytes.Contains(w.buf, []byte(`\documentclass`)) {
				w.log.Info(`found \documentclass`)
				w.file.flags |= flagDocumentClass
			}
		}
	}

	return w.wc.Close()
}

// A Document outlines the methods needed to create a PDF file from TeX
// sources, within the context of TeX.
type Document interface {
	// WorkingDirectory returns the path to a random directory, for
	// AddFile and NewWriter to place new files in it. Compilation will
	// usually happen by changing directory into it.
	//
	// On the first invocation, this will try to create a new, randomly
	// named directory.
	WorkingDirectory() (string, error)

	// AddFile saves the given content as a file in the document's
	// working directory, with the given name.
	//
	// The name is subject to strict validation, any deviation from the
	// rules will end in an InputError:
	//	- no duplicate files,
	//	- no funny characters,
	//	- only relative paths,
	//	- no directory traversal.
	// Additional rules may be imposed by the underlying file system.
	AddFile(name, contents string) error

	// NewWriter allows adding new files using an io.Writer. You MUST
	// call Close() on the returned handle.
	//
	// The name has the same restrictions as outlined in AddFile.
	NewWriter(name string) (io.WriteCloser, error)

	// Cleanup removes the working directory and any contents. You need
	// to read/copy the result PDF with GetResult() before cleaning up.
	Cleanup() error

	// Image declares which Docker image should be used when compiling
	// the sources. Optional and only relevant, when using the Docker
	// executor.
	Image() string

	// Engine defines the LaTeX engine to compile the document with.
	Engine() Engine

	// SetMainInput marks a previously added file (either through AddFile
	// or NewWriter) as main input file ("jobname") for LaTeX.
	//
	// It returns an error, if the file naming rules are violated (see
	// AddFile), or if it references an unknown file. In both cases,
	// no internal state is updated.
	//
	// On success, the MainInput() method will directly return the
	// given name, and stop guessing the main input file.
	SetMainInput(name string) error

	// MainInput tries to guess the main input file for the LaTeX
	// compiler. Candidates taken from .tex files in the root working
	// directory:
	//	- highest precedence have files starting with a "%!texd" mark
	//	- if none of those exists, use files with a \documentclass in the
	//	  first 1 KiB
	//	- if none of those exists, assume any remaining file could be
	//	  a main input file.
	// If in any step only one candidate is found, this return its name,
	// and an error otherwise.
	MainInput() (string, error)

	// GetResult returns a handle to read the compiled PDF. If MainInput()
	// returns an error, GetResult will wrap it in an InputError. If the
	// PDF file does not exist, GetResult will return a CompilationError.
	GetResult() (io.ReadCloser, error)

	// GetLogs returns a handle to read the TeX compiler logs. If MainInput()
	// returns an error, GetLogs will wrap it in an InputError. If the
	// log file does not exist, GetLogs will return a CompilationError.
	GetLogs() (io.ReadCloser, error)

	// Metrics reports file sizes.
	Metrics() Metrics
}

type document struct {
	fs afero.Fs // when nil, use osfs

	workdir   string
	files     map[string]*File
	mainInput string // only present after SetMainInput(), otherwise ask MainInput()

	log    *zap.Logger
	image  string
	engine Engine

	mkWorkDir    *sync.Once
	mkWorkDirErr error
}

var _ Document = (*document)(nil)

func NewDocument(log *zap.Logger, engine Engine, image string) Document {
	return &document{
		fs:        texFs,
		files:     make(map[string]*File),
		log:       log,
		image:     image,
		engine:    engine,
		mkWorkDir: &sync.Once{},
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

func (doc *document) AddFile(name, contents string) error {
	wc, err := doc.NewWriter(name)
	if err != nil {
		return err
	}
	defer func() { _ = wc.Close() }()

	log := doc.log.With(zap.String("filename", name))
	log.Info("adding file")

	_, err = wc.Write([]byte(contents))
	if err != nil {
		return InputError("cannot save file", err, nil)
	}

	return nil
}

func (doc *document) NewWriter(name string) (wc io.WriteCloser, err error) {
	file := &File{}

	defer func() {
		// add file name as context to error
		if err != nil {
			if cat, ok := err.(*ErrWithCategory); ok {
				cat.extra = KV{"filename": name}
			}
			// cleanup file list
			if file.name != "" {
				delete(doc.files, file.name)
			}
		}
	}()

	var ok bool
	if file.name, ok = cleanpath(name); !ok {
		err = InputError("invalid file name", nil, nil)
		return
	}

	if _, exists := doc.files[name]; exists {
		err = InputError("duplicate file name", nil, nil)
		return
	} else {
		doc.files[name] = file
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
	if osErr != nil {
		err = InputError("cannot create file", osErr, nil)
		return
	}

	log := doc.log.With(zap.String("filename", file.name))
	if isMainCandidate(file.name) {
		log.Debug("mark as candidate")
		file.flags |= flagCandidate
	}

	return &fileWriter{
		log:  log,
		file: file,
		wc:   f,
		buf:  make([]byte, 1024),
	}, nil
}

func (doc *document) SetMainInput(name string) error {
	name, ok := cleanpath(name)
	if !ok {
		return InputError("invalid file name", nil, nil)
	}
	_, ok = doc.files[name]
	if !ok {
		return InputError("unknown input file name", nil, nil)
	}

	doc.log.Info("setting main input", zap.String("filename", name))
	doc.mainInput = name
	return nil
}

func (doc *document) MainInput() (string, error) {
	if doc.mainInput != "" {
		return doc.mainInput, nil
	}

	var withDocClass, withMark, others []*File

	for _, f := range doc.files {
		if f.hasTexdMark() {
			withMark = append(withMark, f)
		} else if f.hasDocumentClass() {
			withDocClass = append(withDocClass, f)
		} else if f.isCandidate() {
			others = append(others, f)
		}
	}

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
	doc.log.Debug("fetching result")
	return doc.openFile(".pdf")
}

func (doc *document) GetLogs() (io.ReadCloser, error) {
	doc.log.Debug("fetching logs")
	return doc.openFile(".log")
}

func cleanpath(name string) (clean string, ok bool) {
	clean = path.Clean(name)
	if /* current directory */ clean == "." ||
		/* forbidden file name */ isForbidden(name) ||
		/* directory traversal */ strings.HasPrefix(clean, "..") ||
		/* absolute paths */ strings.HasPrefix(clean, "/") ||
		/* easily abusable TeX chars */ strings.ContainsAny(clean, "\\%$_^&`") {
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
	if doc.workdir != "" {
		if err := doc.fs.RemoveAll(doc.workdir); err != nil {
			return InputError("cleanup failed", err, nil)
		}
		doc.workdir = ""
	}
	return nil
}

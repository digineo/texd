package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/digineo/texd/exec"
	"github.com/digineo/texd/refstore"
	"github.com/digineo/texd/refstore/dir"
	"github.com/docker/go-units"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type testSuite struct {
	suite.Suite
	svc    *service
	stop   func(context.Context) error
	logger *zap.Logger

	mock mockParams
}

// Parameters for exec.Mock.
type mockParams struct {
	shouldFail     bool
	resultContents string
}

func TestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(testSuite))
}

func (suite *testSuite) SetupSuite() {
	require := suite.Require()

	logger, err := zap.NewDevelopment()
	suite.Require().NoError(err)
	suite.logger = logger

	suite.svc = newService(Options{
		QueueLength:    runtime.GOMAXPROCS(0),
		QueueTimeout:   time.Second,
		MaxJobSize:     units.MiB,
		CompileTimeout: 10 * time.Second,
		Mode:           "local",
		Executor:       suite.Executor,
	}, logger)

	stop, err := suite.svc.start(":2201")
	require.NoError(err)

	suite.stop = stop
}

func (suite *testSuite) Executor(doc exec.Document) exec.Exec {
	return exec.Mock(suite.mock.shouldFail, suite.mock.resultContents)(doc)
}

func (suite *testSuite) TearDownSuite() {
	suite.logger.Debug("tear down suite")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	suite.Require().NoError(suite.stop(ctx))
}

type serviceTestCase struct {
	refstore refstore.Adapter
	files    func(*multipart.Writer) error

	statusCode   int
	query        string // raw query params, without leading "?"
	expectedMIME string
	expectedBody string

	mockParams
}

const (
	mockPDF = "%PDF1.5\n...\n"
	mockLog = "This is MockTeX version 3.14159\n! missing input file\nBye!\n"
)

var serviceTestCases = map[string]serviceTestCase{
	"single file": {
		files:        addDirectory("../testdata/simple", nil),
		statusCode:   http.StatusOK,
		mockParams:   mockParams{false, mockPDF},
		expectedMIME: mimeTypePDF,
		expectedBody: mockPDF,
	},
	"single file, explicit input": {
		files:        addDirectory("../testdata/simple", nil),
		statusCode:   http.StatusOK,
		mockParams:   mockParams{false, mockPDF},
		query:        "input=input.tex",
		expectedMIME: mimeTypePDF,
		expectedBody: mockPDF,
	},
	"single file, explicit unknown input": {
		files:        addDirectory("../testdata/simple", nil),
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockLog},
		query:        "input=nonexistent.tex",
		expectedMIME: mimeTypeJSON,
		expectedBody: `{"category":"input","error":"unknown input file name"}`,
	},
	"single file, unknown engine": {
		files:        addDirectory("../testdata/simple", nil),
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockPDF},
		query:        "engine=dings",
		expectedMIME: mimeTypeJSON,
		expectedBody: `{"category":"input","error":"unknown engine"}`,
	},
	"multiple files": {
		files:        addDirectory("../testdata/multi", nil),
		statusCode:   http.StatusOK,
		mockParams:   mockParams{false, mockPDF},
		expectedMIME: mimeTypePDF,
		expectedBody: mockPDF,
	},
	"missing input file, JSON errors": {
		files:        addDirectory("../testdata/missing", nil),
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockLog},
		expectedMIME: mimeTypeJSON,
		expectedBody: `{"args":["-cd","-silent","-pv-","-pvc-","-pdfxe","input.tex"],"category":"compilation","cmd":"latexmk","error":"compilation failed"}`,
	},
	"missing, with full errors": {
		files:        addDirectory("../testdata/missing", nil),
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockLog},
		query:        "errors=full",
		expectedMIME: mimeTypePlain,
		expectedBody: mockLog,
	},
	"missing, with condensed errors": {
		files:        addDirectory("../testdata/missing", nil),
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockLog},
		query:        "errors=condensed",
		expectedMIME: mimeTypePlain,
		expectedBody: "missing input file",
	},
	"missing, different engine": {
		files:        addDirectory("../testdata/missing", nil),
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockLog},
		query:        "engine=lualatex",
		expectedMIME: mimeTypeJSON,
		expectedBody: `{"args":["-cd","-silent","-pv-","-pvc-","-pdflua","input.tex"],"category":"compilation","cmd":"latexmk","error":"compilation failed"}`,
	},
	"reference store, store file": {
		files: addDirectory("../testdata/reference", map[string]refAction{
			"preamble.sty": refStore,
		}),
		statusCode:   http.StatusOK,
		mockParams:   mockParams{false, mockPDF},
		expectedMIME: mimeTypePDF,
		expectedBody: mockPDF,
	},
	"reference store, use unknown file": {
		files: addDirectory("../testdata/reference", map[string]refAction{
			"preamble.sty": refUse,
		}),
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockPDF},
		expectedMIME: mimeTypeJSON,
		expectedBody: `{"category":"reference","error":"unknown file references","references":["sha256:p5w-x0VQUh2kXyYbbv1ubkc-oZ0z7aZYNjSKVVzaZuo"]}`,
	},
	"reference store, invalid reference": {
		files: addDirectory("../testdata/reference", map[string]refAction{
			"preamble.sty": refUseInvalid,
		}),
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockPDF},
		expectedMIME: mimeTypeJSON,
		expectedBody: `{"category":"input","content-type":"application/x.texd; ref=use","error":"failed to parse reference","name":"preamble.sty","part":1}`,
	},
	"reference store, use known file": {
		refstore: func() refstore.Adapter {
			refs, err := dir.NewMemory(&url.URL{Path: "/deeply/nested"})
			if err != nil {
				panic(err)
			}
			contents, err := os.Open("../testdata/reference/preamble.sty")
			if err != nil {
				panic(err)
			}
			defer contents.Close()
			if err = refs.Store(zap.NewNop(), contents); err != nil {
				panic(err)
			}
			return refs
		}(),
		files: addDirectory("../testdata/reference", map[string]refAction{
			"preamble.sty": refUse,
		}),
		statusCode:   http.StatusOK,
		mockParams:   mockParams{false, mockPDF},
		expectedMIME: mimeTypePDF,
		expectedBody: mockPDF,
	},
}

func (suite *testSuite) TestService() {
	for name := range serviceTestCases {
		tc := serviceTestCases[name]
		suite.Run(name, func() {
			suite.runServiceTestCase(tc)
		})
	}
}

func (suite *testSuite) runServiceTestCase(testCase serviceTestCase) {
	assert := suite.Assert()
	require := suite.Require()

	suite.mock = testCase.mockParams

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	if testCase.files != nil {
		require.NoError(testCase.files(w))
	}
	w.Close()

	uri, err := url.Parse("http://localhost:2201/render")
	require.NoError(err)
	uri.RawQuery = testCase.query

	if testCase.refstore != nil {
		cur := suite.svc.refs
		suite.svc.refs = testCase.refstore
		defer func() { suite.svc.refs = cur }()
	}

	req, err := http.NewRequest(http.MethodPost, uri.String(), &b)
	require.NoError(err)

	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	require.NoError(err)

	body, err := io.ReadAll(res.Body)
	require.NoError(err)
	res.Body.Close()

	assert.Equal(testCase.expectedMIME, res.Header.Get("Content-Type"))
	if !assert.Equal(testCase.statusCode, res.StatusCode) {
		suite.logger.Error("unexpected result", zap.ByteString("body", body))
	}
	assert.EqualValues(
		strings.TrimSpace(testCase.expectedBody),
		strings.TrimSpace(string(body)))
}

// Appends all files from a folder.
func addDirectory(dir string, refs map[string]refAction) func(w *multipart.Writer) error {
	return func(w *multipart.Writer) error {
		return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				name := strings.TrimPrefix(path, dir+"/")
				var ref refAction
				if refs != nil {
					ref = refs[name]
				}
				return addFile(w, dir, name, ref)
			}
			return nil
		})
	}
}

type refAction byte

const (
	refNone refAction = iota
	refStore
	refUse
	refUseInvalid
)

func addFile(w *multipart.Writer, dir, name string, ref refAction) error {
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	fw, err := createFormField(w, name, ref)
	if err != nil {
		return err
	}

	switch ref {
	case refNone, refStore:
		_, err = io.Copy(fw, f)
		return err
	case refUse:
		id, err := refstore.ReadIdentifier(f)
		if err != nil {
			return err
		}
		buf := bytes.NewBufferString(id.String())
		_, err = io.Copy(fw, buf)
		return err
	case refUseInvalid:
		buf := bytes.NewBufferString("sha256:asdf")
		_, err = io.Copy(fw, buf)
		return err
	}
	panic("not reached")
}

// taken from GOROOT/src/mime/multipart/writer.go
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// taken from GOROOT/src/mime/multipart/writer.go
func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// taken from GOROOT/src/mime/multipart/writer.go
func createFormField(w *multipart.Writer, name string, ref refAction) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(name), escapeQuotes(name)))
	switch ref {
	case refUse, refUseInvalid:
		h.Set("Content-Type", fmt.Sprintf("%s; ref=use", mimeTypeTexd))
	case refStore:
		h.Set("Content-Type", fmt.Sprintf("%s; ref=store", mimeTypeTexd))
	default:
		h.Set("Content-Type", "application/octet-stream")
	}
	return w.CreatePart(h)

}

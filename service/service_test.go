package service

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/digineo/texd/exec"
	"github.com/digineo/texd/tex"
	"github.com/docker/go-units"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type testSuite struct {
	suite.Suite
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

	stop, err := Start(Options{
		Addr:           ":2201",
		QueueLength:    runtime.GOMAXPROCS(0),
		QueueTimeout:   time.Second,
		MaxJobSize:     units.MiB,
		CompileTimeout: 10 * time.Second,
		Mode:           "local",
		Executor:       suite.Executor,
	}, logger)
	require.NoError(err)

	suite.stop = stop
}

func (suite *testSuite) Executor(doc tex.Document) exec.Exec {
	return exec.Mock(suite.mock.shouldFail, suite.mock.resultContents)(doc)
}

func (suite *testSuite) TearDownSuite() {
	suite.logger.Debug("tear down suite")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	suite.Require().NoError(suite.stop(ctx))
}

type serviceTestCase struct {
	folder       string
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
		folder:       "simple",
		statusCode:   http.StatusOK,
		mockParams:   mockParams{false, mockPDF},
		expectedMIME: mimeTypePDF,
		expectedBody: mockPDF,
	},
	"single file, explicit input": {
		folder:       "simple",
		statusCode:   http.StatusOK,
		mockParams:   mockParams{false, mockPDF},
		query:        "input=input.tex",
		expectedMIME: mimeTypePDF,
		expectedBody: mockPDF,
	},
	"single file, explicit unknown input": {
		folder:       "simple",
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockLog},
		query:        "input=nonexistent.tex",
		expectedMIME: mimeTypeJSON,
		expectedBody: `{"category":"input","error":"unknown input file name"}`,
	},
	"single file, unknown engine": {
		folder:       "simple",
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockPDF},
		query:        "engine=dings",
		expectedMIME: mimeTypeJSON,
		expectedBody: `{"category":"input","error":"unknown engine"}`,
	},
	"multiple files": {
		folder:       "multi",
		statusCode:   http.StatusOK,
		mockParams:   mockParams{false, mockPDF},
		expectedMIME: mimeTypePDF,
		expectedBody: mockPDF,
	},
	"missing input file, JSON errors": {
		folder:       "missing",
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockLog},
		expectedMIME: mimeTypeJSON,
		expectedBody: `{"args":["-cd","-silent","-pv-","-pvc-","-pdfxe","input.tex"],"category":"compilation","cmd":"latexmk","error":"compilation failed"}`,
	},
	"missing, with full errors": {
		folder:       "missing",
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockLog},
		query:        "errors=full",
		expectedMIME: mimeTypePlain,
		expectedBody: mockLog,
	},
	"missing, with condensed errors": {
		folder:       "missing",
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockLog},
		query:        "errors=condensed",
		expectedMIME: mimeTypePlain,
		expectedBody: "missing input file",
	},
	"missing, different engine": {
		folder:       "missing",
		statusCode:   http.StatusUnprocessableEntity,
		mockParams:   mockParams{true, mockLog},
		query:        "engine=lualatex",
		expectedMIME: mimeTypeJSON,
		expectedBody: `{"args":["-cd","-silent","-pv-","-pvc-","-pdflua","input.tex"],"category":"compilation","cmd":"latexmk","error":"compilation failed"}`,
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
	suite.appendFolder(w, "../testdata/"+testCase.folder)
	w.Close()

	uri, err := url.Parse("http://localhost:2201/render")
	require.NoError(err)
	uri.RawQuery = testCase.query

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
func (suite *testSuite) appendFolder(w *multipart.Writer, folder string) {
	require := suite.Require()

	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			suite.appendFile(w, folder, strings.TrimPrefix(path, folder+"/"))
		}
		return nil
	})

	require.NoError(err)
}

func (suite *testSuite) appendFile(w *multipart.Writer, folder, filename string) {
	require := suite.Require()

	suite.logger.Info("append file " + filename)

	file, err := os.Open(filepath.Join(folder, filename))
	require.NoError(err)
	defer file.Close()

	fw, err := w.CreateFormFile(filename, filename)
	require.NoError(err)

	_, err = io.Copy(fw, file)
	require.NoError(err)
}

package service

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net"
	"net/http"
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
	stop           func(context.Context) error
	logger         *zap.Logger
	stubServerAddr net.Addr // port is random

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
	suite.stop(ctx)
}

func (suite *testSuite) TestService() {
	tests := []struct {
		folder     string
		statusCode int
		mockParams
	}{
		{"simple", http.StatusOK, mockParams{false, "%PDF1.5\n...\n"}},
		{"multi", http.StatusOK, mockParams{false, "%PDF1.5\n...\n"}},
		{"missing", http.StatusUnprocessableEntity, mockParams{true, "! missing input file"}},
	}

	for i := range tests {
		tc := tests[i]
		suite.Run(tc.folder, func() {
			suite.testFolder(tc.folder, tc.statusCode, tc.mockParams)
		})
	}
}

func (suite *testSuite) testFolder(folder string, expectedStatusCode int, mock mockParams) {
	assert := suite.Assert()
	require := suite.Require()

	suite.mock = mock

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	suite.appendFolder(w, "../testdata/"+folder)
	w.Close()

	req, err := http.NewRequest(http.MethodPost, "http://localhost:2201/render", &b)
	require.NoError(err)

	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	require.NoError(err)

	body, err := io.ReadAll(res.Body)
	require.NoError(err)
	res.Body.Close()

	if !assert.Equal(expectedStatusCode, res.StatusCode) {
		suite.logger.Error("unexpected result", zap.ByteString("body", body))
	}
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

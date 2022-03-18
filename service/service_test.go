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
	"github.com/docker/go-units"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type testSuite struct {
	suite.Suite
	stop           func(context.Context) error
	logger         *zap.Logger
	stubServerAddr net.Addr // port is random
}

func TestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(testSuite))
}

func (suite *testSuite) SetupSuite() {
	require := suite.Require()

	cfg := zap.NewDevelopmentConfig()
	logger, err := cfg.Build()
	suite.Require().NoError(err)
	suite.logger = logger

	stop, err := Start(Options{
		Addr:           ":2201",
		QueueLength:    runtime.GOMAXPROCS(0),
		QueueTimeout:   time.Second,
		MaxJobSize:     units.MiB,
		CompileTimeout: 10 * time.Second,
		Mode:           "local",
		Executor:       exec.LocalExec,
	}, logger)
	require.NoError(err)

	suite.stop = stop
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
	}{
		{"simple", http.StatusOK},
		{"multi", http.StatusOK},
		{"missing", http.StatusUnprocessableEntity},
	}

	for _, test := range tests {
		suite.Run(test.folder, func() {
			suite.testFolder(test.folder, test.statusCode)
		})
	}
}

func (suite *testSuite) testFolder(folder string, expectedStatusCode int) {
	assert := suite.Assert()
	require := suite.Require()

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
	res.Body.Close()

	if !assert.Equal(expectedStatusCode, res.StatusCode) {
		suite.logger.Error(string(body))
	}
}

// appends all files from a folder
func (suite *testSuite) appendFolder(w *multipart.Writer, folder string) {
	require := suite.Require()

	err := filepath.Walk(folder,
		func(path string, info os.FileInfo, err error) error {
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

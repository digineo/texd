package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type failResponseWriter struct {
	h    http.Header
	code int
}

func (w *failResponseWriter) Header() http.Header    { return w.h }
func (w *failResponseWriter) WriteHeader(code int)   { w.code = code }
func (failResponseWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type mockClock struct{ t time.Time }

func (m mockClock) Now() time.Time                       { return m.t }
func (m mockClock) NewTicker(time.Duration) *time.Ticker { return nil }

func TestHandleStatus(t *testing.T) {
	svc := &service{
		mode:           "local",
		compileTimeout: 3 * time.Second,
		jobs:           make(chan struct{}, 2),
		log:            zap.NewNop(),
	}

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	rec.Body = &bytes.Buffer{}

	svc.HandleStatus(rec, req)

	res := rec.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)

	var status Status
	err := json.NewDecoder(res.Body).Decode(&status)
	require.NoError(t, err)

	assert.EqualValues(t, Status{
		Version:       "development (development)",
		Mode:          "local",
		Timeout:       3,
		Engines:       []string{"xelatex", "pdflatex", "lualatex"},
		DefaultEngine: "xelatex",
		Queue: queueStatus{
			Length:   0,
			Capacity: 2,
		},
	}, status)
}

func TestHandleStatus_withFailIO(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&buf),
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool { return true }),
	)
	clock := &mockClock{time.Unix(1650000000, 0).UTC()}

	svc := &service{
		mode:           "local",
		compileTimeout: 3 * time.Second,
		jobs:           make(chan struct{}, 2),
		log:            zap.New(core, zap.WithClock(clock)),
	}

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := &failResponseWriter{
		h:    make(http.Header),
		code: -1,
	}

	svc.HandleStatus(rec, req)

	assert.Equal(t, http.StatusOK, rec.code)
	assert.Equal(t, mimeTypeJSON, rec.h.Get("Content-Type"))

	assert.Equal(t, strings.Join([]string{
		"2022-04-15T05:20:00.000Z",
		"ERROR",
		"failed to write response",
		`{"error": "io: read/write on closed pipe"}` + "\n",
	}, "\t"), buf.String())
}

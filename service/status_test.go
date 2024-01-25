package service

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/digineo/texd/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failResponseWriter struct {
	h    http.Header
	code int
}

func (w *failResponseWriter) Header() http.Header    { return w.h }
func (w *failResponseWriter) WriteHeader(code int)   { w.code = code }
func (failResponseWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestHandleStatus(t *testing.T) {
	svc := &service{
		mode:           "local",
		compileTimeout: 3 * time.Second,
		jobs:           make(chan struct{}, 2),
		log:            xlog.NewNop(),
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
	log, err := xlog.New(xlog.TypeText, &buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	require.NoError(t, err)

	svc := &service{
		mode:           "local",
		compileTimeout: 3 * time.Second,
		jobs:           make(chan struct{}, 2),
		log:            log,
	}

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := &failResponseWriter{
		h:    make(http.Header),
		code: -1,
	}

	svc.HandleStatus(rec, req)

	assert.Equal(t, http.StatusOK, rec.code)
	assert.Equal(t, mimeTypeJSON, rec.h.Get("Content-Type"))

	msg := `level=ERROR msg="failed to write response" error="io: read/write on closed pipe"` + "\n"
	assert.Equal(t, msg, buf.String())
}

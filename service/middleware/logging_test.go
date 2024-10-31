package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/digineo/texd/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogging(t *testing.T) {
	t.Parallel()

	var h http.Handler
	h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		w.WriteHeader(http.StatusOK)
	})
	h = RequestID(h)

	var buf bytes.Buffer
	log, err := xlog.New(
		xlog.AsText(),
		xlog.WriteTo(&buf),
		xlog.WithSource(),
		xlog.MockClock(time.Unix(1650000000, 0).UTC()),
	)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	WithLogging(log)(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, strings.Join([]string{
		"[05:20:00.000] INFO logging.go:68",
		"",
		"method=GET",
		"status=200",
		"bytes=0",
		"host=192.0.2.1",
		"url=/",
	}, " ")+"\n", buf.String())
}

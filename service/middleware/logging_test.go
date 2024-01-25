package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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
	log, err := xlog.New(xlog.TypeText, &buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	WithLogging(log)(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)

	msg := `level=INFO msg="" method=GET status=200 bytes=0 host=192.0.2.1 url=/` + "\n"
	assert.Equal(t, msg, buf.String())
}

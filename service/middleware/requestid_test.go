package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware(t *testing.T) {
	t.Parallel()

	var contextID string
	captureContextID := func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		val, ok := GetRequestID(r)
		require.True(t, ok)
		assert.NotEmpty(t, val)
		contextID = val

		w.WriteHeader(http.StatusOK)
	}

	w := httptest.NewRecorder()
	RequestID(http.HandlerFunc(captureContextID)).ServeHTTP(w,
		httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	headerId := w.Header().Get(HeaderKey)
	require.NotEmpty(t, headerId)

	assert.Equal(t, headerId, contextID)
}

func TestRequestIDField(t *testing.T) {
	t.Parallel()

	var ctxIDField slog.Attr
	captureContextID := func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		ctxIDField = RequestIDField(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	w := httptest.NewRecorder()
	RequestID(http.HandlerFunc(captureContextID)).ServeHTTP(w,
		httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	headerId := w.Header().Get(HeaderKey)
	require.NotEmpty(t, headerId)

	assert.Equal(t, "request-id", ctxIDField.Key)
	assert.Equal(t, headerId, ctxIDField.Value.String())
}

func TestRequestIDField_missing(t *testing.T) {
	t.Parallel()

	var ctxIDField slog.Attr
	captureContextID := func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		ctxIDField = RequestIDField(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	w := httptest.NewRecorder()
	http.HandlerFunc(captureContextID).ServeHTTP(w,
		httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	headerId := w.Header().Get(HeaderKey)
	require.Empty(t, headerId)

	assert.Equal(t, slog.Attr{}, ctxIDField)
}

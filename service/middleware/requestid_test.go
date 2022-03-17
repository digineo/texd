package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware(t *testing.T) {
	t.Parallel()

	var contextId string
	captureContextID := func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		val, ok := r.Context().Value(ContextKey).(string)
		require.True(t, ok)
		assert.NotEmpty(t, val)
		contextId = val

		w.WriteHeader(http.StatusOK)
	}

	w := httptest.NewRecorder()
	RequestID(http.HandlerFunc(captureContextID)).ServeHTTP(w,
		httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	headerId := w.Header().Get(HeaderKey)
	require.NotEmpty(t, headerId)

	assert.Equal(t, headerId, contextId)
}

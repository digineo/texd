package service

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleUI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	rec.Body = &bytes.Buffer{}

	HandleUI(rec, req)

	res := rec.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, mimeTypeHTML, res.Header.Get("Content-Type"))

	var buf bytes.Buffer
	_, err := io.Copy(&buf, res.Body)
	require.NoError(t, err)
	assert.EqualValues(t, string(uiHTML), buf.String())
}

package middleware

import (
	"bytes"
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

type mockClock struct{ t time.Time }

func (m mockClock) Now() time.Time                       { return m.t }
func (m mockClock) NewTicker(time.Duration) *time.Ticker { return nil }

func TestLogging(t *testing.T) {
	t.Parallel()

	var h http.Handler
	h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		w.WriteHeader(http.StatusOK)
	})
	h = RequestID(h)

	var buf bytes.Buffer
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&buf),
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool { return true }),
	)
	clock := &mockClock{time.Unix(1650000000, 0).UTC()}
	log := zap.New(core, zap.WithClock(clock))

	w := httptest.NewRecorder()
	WithLogging(log)(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)

	require.NoError(t, log.Sync())

	msg := strings.Join([]string{
		"2022-04-15T05:20:00.000Z",
		"INFO",
		"",
		`{"method": "GET", "status": 200, "bytes": 0, "host": "192.0.2.1", "url": "/"}` + "\n",
	}, "\t")

	assert.Equal(t, msg, buf.String())
}

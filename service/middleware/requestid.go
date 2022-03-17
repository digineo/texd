package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

const HeaderKey = "X-Request-Id"

type contextKey string

const ContextKey = contextKey("request-id")

func generateRequestId() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("rand error: %w", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateRequestId()
		r = r.WithContext(context.WithValue(r.Context(), ContextKey, id))
		w.Header().Set(HeaderKey, id)

		next.ServeHTTP(w, r)
	})
}

func GetRequestID(r *http.Request) (string, bool) {
	id, ok := r.Context().Value(ContextKey).(string)
	return id, ok
}

func RequestIDField(ctx context.Context) zap.Field {
	if id, ok := ctx.Value(ContextKey).(string); ok {
		return zap.String("request-id", id)
	}
	return zap.Skip()
}

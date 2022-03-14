package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

const HeaderKey = "X-Request-Id"

type contextKey string

const ContextKey = contextKey("request-id")

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.Must(uuid.NewRandom()).String()
		r = r.WithContext(context.WithValue(r.Context(), ContextKey, id))
		w.Header().Set(HeaderKey, id)

		next.ServeHTTP(w, r)
	})
}

func GetRequestID(r *http.Request) (string, bool) {
	id, ok := r.Context().Value(ContextKey).(string)
	return id, ok
}

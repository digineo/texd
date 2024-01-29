package middleware

import (
	"log/slog"
	"net"
	"net/http"

	"github.com/digineo/texd/xlog"
)

type responseLogger struct {
	status, n int
	http.ResponseWriter
}

func (l *responseLogger) Write(b []byte) (n int, err error) {
	n, err = l.ResponseWriter.Write(b)
	l.n += n
	return
}

func (l *responseLogger) WriteHeader(status int) {
	l.ResponseWriter.WriteHeader(status)
	l.status = status
}

// Logging performs request logging. This method takes heavy inspiration
// from (github.com/gorilla/handlers).CombinedLoggingHandler.
func WithLogging(log xlog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rl := &responseLogger{ResponseWriter: w}
			url := *r.URL

			next.ServeHTTP(rl, r)

			logAttrs := []slog.Attr{
				RequestIDField(r.Context()),
				xlog.String("method", r.Method),
				xlog.Int("status", rl.status),
				xlog.Int("bytes", rl.n),
			}

			if url.User != nil {
				if name := url.User.Username(); name != "" {
					logAttrs = append(logAttrs, xlog.String("username", name))
				}
			}

			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			logAttrs = append(logAttrs, xlog.String("host", host))

			// Requests using the CONNECT method over HTTP/2.0 must use
			// the authority field (aka r.Host) to identify the target.
			// Refer: https://httpwg.github.io/specs/rfc7540.html#CONNECT
			uri := r.RequestURI
			if r.ProtoMajor == 2 && r.Method == "CONNECT" {
				uri = r.Host
			}
			if uri == "" {
				uri = url.RequestURI()
			}
			logAttrs = append(logAttrs, xlog.String("url", uri))

			log.Info("", logAttrs...)
		})
	}
}

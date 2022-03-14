package middleware

import (
	"bytes"
	"net"
	"net/http"
	"os"
	"strconv"
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
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rl := &responseLogger{ResponseWriter: w}
		url := *r.URL

		next.ServeHTTP(rl, r)

		username := "-"
		if url.User != nil {
			if name := url.User.Username(); name != "" {
				username = name
			}
		}

		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}

		uri := r.RequestURI

		// Requests using the CONNECT method over HTTP/2.0 must use
		// the authority field (aka r.Host) to identify the target.
		// Refer: https://httpwg.github.io/specs/rfc7540.html#CONNECT
		if r.ProtoMajor == 2 && r.Method == "CONNECT" {
			uri = r.Host
		}
		if uri == "" {
			uri = url.RequestURI()
		}

		id, haveID := GetRequestID(r)

		var buf bytes.Buffer
		buf.Grow(len(id) + len(r.Method) + len(host) + len(username) + len(uri) + 50) // reduce allocs
		if haveID {
			buf.WriteString(id)
			buf.WriteString(": ")
		}
		buf.WriteString(host)
		buf.WriteString(" - ")
		buf.WriteString(username)
		buf.WriteString(" - ")
		buf.WriteString(r.Method)
		buf.WriteByte(' ')
		buf.WriteString(uri)
		buf.WriteString(" - ")
		buf.WriteString(strconv.Itoa(rl.status))
		buf.WriteByte(' ')
		buf.WriteString(strconv.Itoa(rl.n))
		buf.WriteByte('\n')
		_, _ = buf.WriteTo(os.Stderr)
	})
}

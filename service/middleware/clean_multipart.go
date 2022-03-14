package middleware

import (
	"log"
	"net/http"
)

func CleanMultipart(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		if r.MultipartForm == nil {
			return
		}
		if err := r.MultipartForm.RemoveAll(); err != nil {
			id, _ := GetRequestID(r)
			log.Printf("%s: failed to cleanup multipart/form-data residues: %v", id, err)
		}
	})
}

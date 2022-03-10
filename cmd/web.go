package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/dmke/texd/exec"
	"github.com/dmke/texd/requestid"
	"github.com/dmke/texd/tex"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

const (
	mimeTypeJSON = "application/json; charset=utf-8"
	mimeTypePDF  = "application/pdf"
)

func StartWeb(addr string, queueLen int, executor func(tex.Document) exec.Exec) func(context.Context) error {
	r := mux.NewRouter()

	renderer := newRenderer(queueLen, executor)

	r.Handle("/render", renderer).Methods(http.MethodPost)
	r.HandleFunc("/metrics", handleMetrics).Methods(http.MethodGet)

	// r.Use(handlers.RecoveryHandler())
	r.Use(requestid.Middleware)
	r.Use(handlers.CompressHandler)

	srv := http.Server{
		Addr:    addr,
		Handler: handlers.CombinedLoggingHandler(os.Stdout, r),
	}

	go func() {
		log.Printf("starting server on %q", addr)
		if err := srv.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				log.Printf("shutting down server on %q", addr)
			} else {
				log.Printf("error starting server on %q: %v", addr, err)
			}
		}
	}()

	return func(ctx context.Context) error {
		renderer.Close()
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		return nil
	}
}

// TODO
func handleMetrics(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
}

func errorResponse(res http.ResponseWriter, err error) {
	res.Header().Set("Content-Type", mimeTypeJSON)
	res.Header().Set("X-Content-Type-Options", "nosniff")
	res.WriteHeader(http.StatusUnprocessableEntity)

	if cat, ok := err.(*tex.ErrWithCategory); ok {
		json.NewEncoder(res).Encode(cat)
	} else {
		json.NewEncoder(res).Encode(map[string]string{
			"error":    "internal server error",
			"category": "internal",
		})
	}
}

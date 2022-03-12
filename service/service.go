package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dmke/texd/exec"
	"github.com/dmke/texd/requestid"
	"github.com/dmke/texd/tex"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

const (
	mimeTypeJSON  = "application/json; charset=utf-8"
	mimeTypePDF   = "application/pdf"
	mimeTypePlain = "text/plain; charset=utf-8"
	mimeTypeHTML  = "text/html; charset=utf-8"
)

type Options struct {
	Addr           string
	QueueLength    int
	QueueTimeout   time.Duration
	Executor       func(tex.Document) exec.Exec
	CompileTimeout time.Duration
	Mode           string
	Images         []string
}

type service struct {
	mode   string
	images []string

	jobs           chan struct{}
	executor       func(tex.Document) exec.Exec
	compileTimeout time.Duration
	queueTimeout   time.Duration
}

func Start(opts Options) func(context.Context) error {
	svc := &service{
		mode:           opts.Mode,
		jobs:           make(chan struct{}, opts.QueueLength),
		executor:       opts.Executor,
		compileTimeout: opts.CompileTimeout,
		queueTimeout:   opts.QueueTimeout,
		images:         opts.Images,
	}

	r := mux.NewRouter()
	r.HandleFunc("/", HandleUI).Methods(http.MethodGet)
	r.PathPrefix("/assets/").Handler(HandleAssets()).Methods(http.MethodGet)

	r.HandleFunc("/render", svc.HandleRender).Methods(http.MethodPost)
	r.HandleFunc("/status", svc.HandleStatus).Methods(http.MethodGet)
	r.HandleFunc("/metrics", svc.HandleMetrics).Methods(http.MethodGet)

	// r.Use(handlers.RecoveryHandler())
	r.Use(requestid.Middleware)
	r.Use(handlers.CompressHandler)

	srv := http.Server{
		Addr:    opts.Addr,
		Handler: handlers.CombinedLoggingHandler(os.Stdout, r),
	}

	go func() {
		log.Printf("starting server on %q", opts.Addr)
		if err := srv.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				log.Printf("shutting down server on %q", opts.Addr)
			} else {
				log.Printf("error starting server on %q: %v", opts.Addr, err)
			}
		}
	}()

	return func(ctx context.Context) error {
		svc.Close()
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		return nil
	}
}

// TODO: collect metrics for prometheus.
func (svc *service) HandleMetrics(res http.ResponseWriter, req *http.Request) {
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

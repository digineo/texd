package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/digineo/texd/exec"
	"github.com/digineo/texd/service/middleware"
	"github.com/digineo/texd/tex"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
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

	log *zap.Logger
}

func Start(opts Options, log *zap.Logger) func(context.Context) error {
	svc := &service{
		mode:           opts.Mode,
		jobs:           make(chan struct{}, opts.QueueLength),
		executor:       opts.Executor,
		compileTimeout: opts.CompileTimeout,
		queueTimeout:   opts.QueueTimeout,
		images:         opts.Images,
		log:            log,
	}

	r := mux.NewRouter()
	r.HandleFunc("/", HandleUI).Methods(http.MethodGet)
	r.PathPrefix("/assets/").Handler(HandleAssets()).Methods(http.MethodGet)

	r.HandleFunc("/render", svc.HandleRender).Methods(http.MethodPost)
	r.HandleFunc("/status", svc.HandleStatus).Methods(http.MethodGet)
	r.HandleFunc("/metrics", svc.HandleMetrics).Methods(http.MethodGet)

	// r.Use(handlers.RecoveryHandler())
	r.Use(middleware.RequestID)
	r.Use(handlers.CompressHandler)
	r.Use(middleware.WithLogging(log))
	r.Use(middleware.CleanMultipart)

	srv := http.Server{
		Addr:    opts.Addr,
		Handler: r,
	}

	go func() {
		zaddr := zap.String("addr", opts.Addr)

		log.Info("starting server", zaddr)
		if err := srv.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				log.Info("shutting down server", zaddr)
			} else {
				log.Error("error starting server", zaddr, zap.Error(err))
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

var discardlog = zap.NewNop()

func (svc *service) Logger() *zap.Logger {
	if svc.log == nil {
		return discardlog
	}
	return svc.log
}

// TODO: collect metrics for prometheus.
func (svc *service) HandleMetrics(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
}

func errorResponse(log *zap.Logger, res http.ResponseWriter, err error) {
	res.Header().Set("Content-Type", mimeTypeJSON)
	res.Header().Set("X-Content-Type-Options", "nosniff")
	res.WriteHeader(http.StatusUnprocessableEntity)

	var respErr error
	if cat, ok := err.(*tex.ErrWithCategory); ok {
		respErr = json.NewEncoder(res).Encode(cat)
	} else {
		respErr = json.NewEncoder(res).Encode(map[string]string{
			"error":    "internal server error",
			"category": "internal",
		})
	}
	if respErr != nil {
		log.Error("failed to write response", zap.Error(respErr))
	}
}

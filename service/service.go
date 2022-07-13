package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/digineo/texd/docs"
	"github.com/digineo/texd/exec"
	"github.com/digineo/texd/metrics"
	"github.com/digineo/texd/refstore"
	"github.com/digineo/texd/refstore/nop"
	"github.com/digineo/texd/service/middleware"
	"github.com/digineo/texd/tex"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const (
	mimeTypeJSON  = "application/json; charset=utf-8"
	mimeTypePDF   = "application/pdf"
	mimeTypePlain = "text/plain; charset=utf-8"
	mimeTypeHTML  = "text/html; charset=utf-8"
	mimeTypeTexd  = "application/x.texd"

	KeepJobsNever = iota
	KeepJobsAlways
	KeepJobsOnFailure
)

type Options struct {
	Addr           string
	QueueLength    int
	QueueTimeout   time.Duration
	MaxJobSize     int64 // number of bytes
	Executor       func(exec.Document) exec.Exec
	CompileTimeout time.Duration
	Mode           string
	KeepJobs       int // used for debugging
	Images         []string
	RefStore       refstore.Adapter
}

type service struct {
	mode   string
	images []string
	refs   refstore.Adapter

	jobs           chan struct{}
	executor       func(exec.Document) exec.Exec
	compileTimeout time.Duration
	queueTimeout   time.Duration
	maxJobSize     int64 // number of bytes
	keepJobs       int

	log *zap.Logger
}

func newService(opts Options, log *zap.Logger) *service {
	svc := &service{
		mode:           opts.Mode,
		jobs:           make(chan struct{}, opts.QueueLength),
		executor:       opts.Executor,
		compileTimeout: opts.CompileTimeout,
		queueTimeout:   opts.QueueTimeout,
		maxJobSize:     opts.MaxJobSize,
		keepJobs:       opts.KeepJobs,
		images:         opts.Images,
		refs:           opts.RefStore,
		log:            log,
	}
	if svc.queueTimeout <= 0 {
		svc.queueTimeout = time.Second
	}
	if svc.refs == nil {
		svc.refs, _ = nop.New(nil, nil)
	}
	return svc
}

func (svc *service) routes() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/", HandleUI).Methods(http.MethodGet)
	r.PathPrefix("/assets/").Handler(HandleAssets()).Methods(http.MethodGet)
	r.PathPrefix("/docs").Handler(http.StripPrefix("/docs", docs.Handler())).Methods(http.MethodGet)

	render := http.Handler(http.HandlerFunc(svc.HandleRender))
	if max := svc.maxJobSize; max > 0 {
		render = http.MaxBytesHandler(render, max)
	}
	r.Handle("/render", render).Methods(http.MethodPost)

	r.HandleFunc("/status", svc.HandleStatus).Methods(http.MethodGet)
	r.Handle("/metrics", svc.newMetricsHandler()).Methods(http.MethodGet)

	// r.Use(handlers.RecoveryHandler())
	r.Use(middleware.RequestID)
	r.Use(handlers.CompressHandler)
	r.Use(middleware.WithLogging(svc.log))
	r.Use(middleware.CleanMultipart)
	return r
}

func (svc *service) start(addr string) (func(context.Context) error, error) {
	srv := http.Server{
		Addr:    addr,
		Handler: svc.routes(),
	}

	zaddr := zap.String("addr", addr)
	svc.Logger().Info("starting server", zaddr)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	go func() {
		if e := srv.Serve(l); !errors.Is(e, http.ErrServerClosed) {
			svc.Logger().Error("unexpected HTTP server shutdown", zap.Error(err))
		}
	}()

	return func(ctx context.Context) error {
		svc.Close()
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		return nil
	}, nil
}

func Start(opts Options, log *zap.Logger) (func(context.Context) error, error) {
	return newService(opts, log).start(opts.Addr)
}

var discardlog = zap.NewNop()

func (svc *service) Logger() *zap.Logger {
	if svc.log == nil {
		return discardlog
	}
	return svc.log
}

func (svc *service) newMetricsHandler() http.Handler {
	prom := promhttp.Handler()

	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		qlen, qcap := float64(len(svc.jobs)), float64(cap(svc.jobs))
		metrics.JobsQueueLength.Set(qlen)
		metrics.JobQueueRatio.Set(qlen / qcap)

		metrics.Info.WithLabelValues(svc.mode).Set(1)

		prom.ServeHTTP(res, req)
	})
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

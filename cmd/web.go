package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/dmke/texd/requestid"
	"github.com/dmke/texd/tex"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func StartWeb(addr string, queueLen int) func(context.Context) error {
	r := mux.NewRouter()

	handleRender, renderQueue := newRenderHandler(queueLen)
	r.Handle("/render", handleRender).Methods(http.MethodPost)
	r.HandleFunc("/metrics", handleMetrics).Methods(http.MethodGet)

	r.Use(handlers.RecoveryHandler())
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
		close(renderQueue)
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		return nil
	}
}

func newRenderHandler(queueLen int) (http.Handler, chan struct{}) {
	queue := make(chan struct{}, queueLen) // TODO: this leaks
	var job struct{}

	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		var err error
		req.ParseMultipartForm(5 << 20)

		params := req.URL.Query()
		// Get name of Docker image. Ignored in localMode, but must be
		// whitelisted otherwise.
		image := "" // TODO: start with DefaultImage
		if img := params.Get("image"); img != "" {
			// TODO: check if qImage is allowed
		}

		// Fetch name of TeX engine. optional, but must be supported
		engine := tex.DefaultEngine
		if eng := params.Get("engine"); eng != "" {
			if engine, err = tex.ParseTeXEngine(eng); err != nil {
				log.Printf("invalid engine: %v", err)
				errorResponse(res, err)
				return
			}
		}

		// Add a new job to the queue and bail if we're over capacity.
		select {
		case queue <- job:
			defer func() { <-queue }()
		default:
			err := tex.QueueError("queue full, please try again later", nil, nil)
			log.Println(err)
			errorResponse(res, err)
			return
		}

		doc := tex.NewDocument(engine, image)
		defer func() {
			if err := doc.Cleanup(); err != nil {
				log.Printf("cleanup failed: %v", err)
			}
		}()
		for name, contents := range req.PostForm {
			// AddFile() will only accept one boby per file name and construct
			// a proper error message. No need to perform something akin to
			// `AddFile(contents[0]) if len(contents) == 1` here.
			for _, content := range contents {
				log.Printf("adding file: %q", name)
				if err := doc.AddFile(name, content); err != nil {
					log.Printf("adding file %q failed: %v", name, err)
					errorResponse(res, err)
					return
				}
			}
		}

		// Optionally, set main input file. When present, the name must be
		// included of multipart request body.
		if input := params.Get("input"); input != "" {
			if err := doc.SetMainInput(input); err != nil {
				log.Printf("invalid main input file %q: %v", input, err)
				errorResponse(res, err)
				return
			}
		}

		// Check presence main input file. If not given, guess from file
		// listing.
		main, err := doc.MainInput()
		if err != nil {
			errorResponse(res, err)
			return
		}
		log.Printf("using main input file: %q", main)

		// TODO: start compiler process

		res.WriteHeader(http.StatusOK)
	}), queue
}

// TODO
func handleMetrics(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
}

func errorResponse(res http.ResponseWriter, err error) {
	res.Header().Set("Content-Type", "application/json; charset=utf-8")
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

package cmd

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/dmke/texd/exec"
	"github.com/dmke/texd/tex"
)

type renderer struct {
	jobs     chan struct{}
	executor func(tex.Document) exec.Exec
}

func newRenderer(queueLen int, executor func(tex.Document) exec.Exec) *renderer {
	return &renderer{
		jobs:     make(chan struct{}, queueLen),
		executor: executor,
	}
}

func (r *renderer) acquire(ctx context.Context) error {
	select {
	case r.jobs <- struct{}{}:
		// success
		return nil
	case <-ctx.Done():
		return tex.QueueError("queue full, please try again later", ctx.Err(), nil)
	default:
		return tex.QueueError("queue full, please try again later", nil, nil)
	}
}

func (r *renderer) release() {
	<-r.jobs
}

func (r *renderer) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if err := r.serveHTTP(res, req); err != nil {
		errorResponse(res, err)
	}
}

func (r *renderer) serveHTTP(res http.ResponseWriter, req *http.Request) error {
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
			return err
		}
	}

	// Add a new job to the queue and bail if we're over capacity.
	// Don't wait too long for other jobs to complete.
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second) // TODO: make configurable
	defer cancel()

	if err = r.acquire(ctx); err != nil {
		log.Println(err)
		return err
	}
	defer r.release()

	doc := tex.NewDocument(engine, image)
	defer func() {
		if err := doc.Cleanup(); err != nil {
			log.Printf("cleanup failed: %v", err)
		}
	}()

	if err := doc.AddFiles(req.PostForm); err != nil {
		log.Printf("adding files failed: %v", err)
		return err
	}

	// Optionally, set main input file. When present, the name must be
	// included of multipart request body.
	if input := params.Get("input"); input != "" {
		if err := doc.SetMainInput(input); err != nil {
			log.Printf("invalid main input file %q: %v", input, err)
			return err
		}
	}

	// Check presence main input file. If not given, guess from file
	// listing.
	main, err := doc.MainInput()
	if err != nil {
		return err
	}
	log.Printf("using main input file: %q", main)

	if err := req.Context().Err(); err != nil {
		log.Printf("cancel render job, client is gone: %v", err)
		return err
	}

	if err := r.executor(doc).Run(ctx); err != nil {
		log.Printf("render job failed: %v", err)
		return err
	}

	pdf, err := doc.GetResult()
	if err != nil {
		log.Printf("failed to get result: %v", err)
		return err
	}
	defer pdf.Close()

	// Send PDF
	res.Header().Set("Content-Type", mimeTypePDF)
	res.WriteHeader(http.StatusOK)
	_, err = io.Copy(res, pdf)
	return err
}

func (r *renderer) Close() {
	close(r.jobs)
}

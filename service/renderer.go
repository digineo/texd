package service

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"net/http"

	"github.com/digineo/texd/service/middleware"
	"github.com/digineo/texd/tex"
	"go.uber.org/zap"
)

func (svc *service) Close() {
	close(svc.jobs)
}

func (svc *service) HandleRender(res http.ResponseWriter, req *http.Request) {
	if svc.compileTimeout > 0 {
		// apply global render timeout
		ctx, cancel := context.WithTimeout(req.Context(), svc.compileTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	log := svc.Logger().With(middleware.RequestIDField(req.Context()))
	if err := svc.render(log, res, req); err != nil {
		errorResponse(log, res, err)
	}
}

func (svc *service) render(log *zap.Logger, res http.ResponseWriter, req *http.Request) error { //nolint:funlen
	err := req.ParseMultipartForm(5 << 20)
	if err != nil {
		return tex.InputError("parsing form data failed", err, nil)
	}
	params := req.URL.Query()

	image, err := svc.validateImageParam(params.Get("image"))
	if err != nil {
		return err
	}

	engine, err := svc.validateEngineParam(params.Get("engine"))
	if err != nil {
		return err
	}

	// Add a new job to the queue and bail if we're over capacity.
	if err = svc.acquire(req.Context()); err != nil {
		log.Error("failed enter queue", zap.Error(err))
		return err
	}
	defer svc.release()

	doc := tex.NewDocument(log, engine, image)
	defer func() {
		if err := doc.Cleanup(); err != nil {
			log.Error("cleanup failed", zap.Error(err))
		}
	}()

	if err := doc.AddFiles(req); err != nil {
		log.Error("failed to add files: %v", zap.Error(err))
		return err
	}

	// Optionally, set main input file. When present, the name must be
	// included of multipart request body.
	if input := params.Get("input"); input != "" {
		if err := doc.SetMainInput(input); err != nil {
			log.Error("invalid main input file",
				zap.String("filename", input),
				zap.Error(err))
			return err
		}
	}

	// Check presence main input file. If not given, guess from file
	// listing.
	if _, err := doc.MainInput(); err != nil {
		return err
	}

	if err := req.Context().Err(); err != nil {
		log.Error("cancel render job, client is gone", zap.Error(err))
		return err
	}

	if err := svc.executor(doc).Run(req.Context(), log); err != nil {
		if format := params.Get("errors"); format != "" {
			logReader, lerr := doc.GetLogs()
			if lerr != nil {
				log.Error("failed to get logs", zap.Error(lerr))
				return err // not lerr, client gets error from executor.Run()
			}
			logfileResponse(log, res, format, logReader)
			return nil // header is already written
		}

		return err
	}

	pdf, err := doc.GetResult()
	if err != nil {
		log.Error("failed to get result", zap.Error(err))
		return err
	}
	defer pdf.Close()

	// Send PDF
	res.Header().Set("Content-Type", mimeTypePDF)
	res.WriteHeader(http.StatusOK)
	if _, err = io.Copy(res, pdf); err != nil {
		log.Error("failed to send results", zap.Error(err))
	}
	return nil // header is already written
}

// Validates name of Docker image. Ignored in local mode, but must be
// allowed otherwise.
func (svc *service) validateImageParam(image string) (string, error) {
	if svc.mode != "container" {
		return "", nil
	}
	if image == "" {
		return svc.images[0], nil
	}
	for _, name := range svc.images {
		if name == image {
			return image, nil
		}
	}
	log.Printf("forbidden image: %q", image)
	return "", tex.InputError("forbidden image name", nil, tex.KV{"image": image})
}

// Validate TeX engine. Optional, but engine must be known to texd.
func (svc *service) validateEngineParam(name string) (engine tex.Engine, err error) {
	if name == "" {
		return tex.DefaultEngine, nil
	}
	engine, err = tex.ParseEngine(name)
	if err != nil {
		log.Printf("invalid engine: %v", err)
		err = tex.InputError("unknown engine", err, nil)
	}
	return
}

func logfileResponse(log *zap.Logger, res http.ResponseWriter, format string, logs io.ReadCloser) {
	res.Header().Set("Content-Type", mimeTypePlain)
	res.WriteHeader(http.StatusUnprocessableEntity)

	if format != "condensed" {
		if _, err := io.Copy(res, logs); err != nil {
			log.Error("failed to send logs", zap.Error(err))
		}
		return
	}

	s := bufio.NewScanner(logs)
	for s.Scan() {
		if line := s.Bytes(); bytes.HasPrefix(line, []byte("!")) {
			// drop error indicator and add line break
			line = append(bytes.TrimLeft(line, "! "), '\n')
			if _, err := res.Write(line); err != nil {
				log.Error("failed to send logs", zap.Error(err))
				return
			}
		}
	}
	if err := s.Err(); err != nil {
		log.Error("failed to read logs", zap.Error(err))
	}
}

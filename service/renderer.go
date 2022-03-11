package service

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"net/http"

	"github.com/dmke/texd/tex"
)

func (svc *service) Close() {
	close(svc.jobs)
}

func (svc *service) HandleRender(res http.ResponseWriter, req *http.Request) {
	if svc.timeout > 0 {
		// apply global render timeout
		ctx, cancel := context.WithTimeout(req.Context(), svc.timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	if err := svc.render(res, req); err != nil {
		errorResponse(res, err)
	}
}

func (svc *service) render(res http.ResponseWriter, req *http.Request) error { //nolint:funlen
	var err error
	req.ParseMultipartForm(5 << 20)
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
		log.Println(err)
		return err
	}
	defer svc.release()

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

	if err := svc.executor(doc).Run(req.Context()); err != nil {
		log.Printf("render job failed: %v", err)

		if format := params.Get("errors"); format != "" {
			logs, lerr := doc.GetLogs()
			if lerr != nil {
				log.Printf("failed to get logs: %v", lerr)
				return err // not lerr, client gets error from executor.Run()
			}
			logfileResponse(res, format, logs)
			return nil // header is already written
		}

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
	if _, err = io.Copy(res, pdf); err != nil {
		log.Printf("failed to send results: %v", err)
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

func logfileResponse(res http.ResponseWriter, format string, logs io.ReadCloser) {
	res.Header().Set("Content-Type", mimeTypePlain)
	res.WriteHeader(http.StatusUnprocessableEntity)

	if format != "condensed" {
		if _, err := io.Copy(res, logs); err != nil {
			log.Printf("failed to send logs: %v", err)
		}
		return
	}

	s := bufio.NewScanner(logs)
	for s.Scan() {
		if line := s.Bytes(); bytes.HasPrefix(line, []byte("!")) {
			// drop error indicator and add line break
			line = append(bytes.TrimLeft(line, "! "), '\n')
			if _, err := res.Write(line); err != nil {
				log.Printf("failed to send logs: %v", err)
				return
			}
		}
	}
	if err := s.Err(); err != nil {
		log.Printf("failed to read logs: %v", err)
	}
}

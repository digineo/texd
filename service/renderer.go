package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"

	"github.com/digineo/texd/refstore"
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

// should we keep the data for a job?
func (svc *service) shouldKeepJobs(err error) bool {
	return svc.keepJobs == KeepJobsAlways || (svc.keepJobs == KeepJobsOnFailure && err != nil)
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
		if svc.shouldKeepJobs(err) {
			return
		}
		if err := doc.Cleanup(); err != nil {
			log.Error("cleanup failed", zap.Error(err))
		}
	}()

	if err := svc.addFiles(log, doc, req); err != nil {
		log.Error("failed to add files", zap.Error(err))
		return err
	}

	// Optionally, set main input file. When present, the name must be
	// included of multipart request body.
	if input := params.Get("input"); input != "" {
		if err = doc.SetMainInput(input); err != nil {
			log.Error("invalid main input file",
				zap.String("filename", input),
				zap.Error(err))
			return err
		}
	}

	// Check presence main input file. If not given, guess from file
	// listing.
	if _, err = doc.MainInput(); err != nil {
		return err
	}

	if err := req.Context().Err(); err != nil {
		log.Error("cancel render job, client is gone", zap.Error(err))
		return err
	}

	if err = svc.executor(doc).Run(req.Context(), log); err != nil {
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

func (svc *service) addFiles(log *zap.Logger, doc tex.Document, req *http.Request) error {
	if mf := req.MultipartForm; mf != nil {
		if err := svc.addFilesFromMultipartForm(log, doc, req, mf); err != nil {
			return err
		}
	}

	for name, contents := range req.PostForm {
		// AddFile() will only accept one body per file name and construct
		// a proper error message. No need to perform something akin to
		// `AddFile(contents[0]) if len(contents) == 1` here.
		for _, content := range contents {
			if err := doc.AddFile(name, content); err != nil {
				return err
			}
		}
	}
	return nil
}

func (svc *service) addFilesFromMultipartForm(log *zap.Logger, doc tex.Document, req *http.Request, mf *multipart.Form) error {
	defer func() { _ = mf.RemoveAll() }()

	var missingReferences []string
	var buf bytes.Buffer

	for name, files := range mf.File {
		name := name
	eachFile:
		for _, f := range files {
			rc, err := f.Open()
			if err != nil {
				return tex.InputError("unable to open file", err, tex.KV{"name": name})
			}
			defer rc.Close()

			buf.Reset()
			if _, err = io.Copy(&buf, rc); err != nil {
				return tex.InputError("failed to read file", err, tex.KV{"name": name})
			}

			target, err := doc.NewWriter(name)
			if err != nil {
				return err // already InputError
			}
			defer target.Close()

			if ct := f.Header.Get("Content-Type"); strings.HasPrefix(ct, mimeTypeTexd) {
				extra := func() tex.KV { return tex.KV{"name": name, "content-type": ct} }
				_, params, err := mime.ParseMediaType(ct)
				if err != nil {
					return tex.InputError("failed to parse content type", err, extra())
				}
				switch ref, ok := params["ref"]; {
				case !ok:
					// no ref param, continue normally
				case ref == "use":
					// buf contains reference hash
					rawID := bytes.TrimSpace(buf.Bytes())
					id, err := refstore.ParseIdentifier(rawID)
					if err != nil {
						return tex.InputError("failed to parse reference", err, extra())
					}

					switch err = svc.refs.CopyFile(svc.log, id, target); {
					case errors.Is(err, refstore.ErrUnknownReference):
						missingReferences = append(missingReferences, string(rawID))
						continue eachFile
					case err != nil:
						return tex.InputError("failed to copy reference", err, extra())
					}
				case ref == "store":
					if err := svc.refs.Store(log, buf.Bytes()); err != nil {
						return tex.InputError("failed to store reference", err, extra())
					}
				default:
					return tex.InputError("invalid ref parameter", err, extra())
				}
			}

			if _, err := io.Copy(target, &buf); err != nil {
				return err
			}
		}
	}

	if len(missingReferences) > 0 {
		sort.Strings(missingReferences)
		return tex.ReferenceError(missingReferences)
	}

	return nil
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

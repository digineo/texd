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
	"time"

	"github.com/digineo/texd/metrics"
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
		metrics.ProcessedFailure.Inc()
		errorResponse(log, res, err)
	}
}

// should we keep the data for a job?
func (svc *service) shouldKeepJobs(err error) bool {
	return svc.keepJobs == KeepJobsAlways || (svc.keepJobs == KeepJobsOnFailure && err != nil)
}

func (svc *service) render(log *zap.Logger, res http.ResponseWriter, req *http.Request) error { //nolint:funlen
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
		metrics.ProcessedRejected.Inc()
		return err
	}
	defer svc.release()

	doc := tex.NewDocument(log, engine, image)
	if id, ok := middleware.GetRequestID(req); ok {
		doc.SetWorkingDirName(id)
	}
	defer func() {
		if svc.shouldKeepJobs(err) {
			return
		}
		observeRenderMetrics(doc)
		if err := doc.Cleanup(); err != nil {
			log.Error("cleanup failed", zap.Error(err))
		}
	}()

	if err := svc.addFiles(log, doc, req); err != nil {
		if tex.IsReferenceError(err) {
			log.Warn("unknown file reference")
		} else {
			log.Error("failed to add files", zap.Error(err))
		}
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
		metrics.ProcessedAborted.Inc()
		return err
	}

	startProcessing := time.Now()
	if err = svc.executor(doc).Run(req.Context(), log); err != nil {
		switch format := params.Get("errors"); format {
		case "full", "condensed":
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
	metrics.ProcessingDuration.Observe(time.Since(startProcessing).Seconds())
	metrics.ProcessedSuccess.Inc()

	pdf, err := doc.GetResult()
	if err != nil {
		log.Error("failed to get result", zap.Error(err))
		return err
	}
	defer func() { _ = pdf.Close() }()

	// Send PDF
	res.Header().Set("Content-Type", mimeTypePDF)
	res.WriteHeader(http.StatusOK)
	n, err := io.Copy(res, pdf)
	if err != nil {
		log.Error("failed to send results", zap.Error(err))
	}
	metrics.OutputSize.Observe(float64(n))
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

type errMissingReference struct{ ref string }

func (err *errMissingReference) Error() string { return err.ref }

func (svc *service) addFiles(log *zap.Logger, doc tex.Document, req *http.Request) error {
	ct := req.Header.Get("Content-Type")
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return tex.InputError("failed to parse request: invalid content type", err, tex.KV{
			"content-type": ct,
		})
	}
	if mt != "multipart/form-data" {
		return tex.InputError("unsupported media type: expected multipart/form-data", nil, tex.KV{
			"media-type": mt,
		})
	}
	mr := multipart.NewReader(req.Body, params["boundary"])
	var missingRefs []string
	refErr := &errMissingReference{}
	for i := 0; ; i++ {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		switch err = svc.addFileFromPart(log, doc, part, i); {
		case errors.As(err, &refErr):
			missingRefs = append(missingRefs, refErr.ref)
		case err != nil:
			return err
		}
	}

	if len(missingRefs) > 0 {
		sort.Strings(missingRefs)
		return tex.ReferenceError(missingRefs)
	}
	return nil
}

func (svc *service) addFileFromPart(log *zap.Logger, doc tex.Document, part *multipart.Part, partNum int) error {
	name := part.FormName()
	if name == "" {
		return tex.InputError("empty name", nil, tex.KV{"part": partNum})
	}

	target, err := doc.NewWriter(name)
	if err != nil {
		tex.ExtendError(err, tex.KV{"part": partNum})
		return err // already InputError
	}
	defer func() { _ = target.Close() }()

	if ct := part.Header.Get("Content-Type"); strings.HasPrefix(ct, mimeTypeTexd) {
		extra := func() tex.KV { return tex.KV{"name": name, "content-type": ct, "part": partNum} }
		_, params, err := mime.ParseMediaType(ct)
		if err != nil {
			return tex.InputError("failed to parse content type", err, extra())
		}

		var buf bytes.Buffer
		if _, err = io.Copy(&buf, part); err != nil {
			return tex.InputError("failed to read part", err, extra())
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
				return &errMissingReference{string(rawID)}
			case err != nil:
				return tex.InputError("failed to use reference", err, extra())
			default:
				return nil // we're done
			}
		case ref == "store":
			tee := io.TeeReader(&buf, target)
			if err := svc.refs.Store(log, tee); err != nil {
				return tex.InputError("failed to store reference", err, extra())
			}
		default:
			return tex.InputError("invalid ref parameter", err, extra())
		}
	}

	if _, err := io.Copy(target, part); err != nil {
		return err
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

func observeRenderMetrics(doc tex.Document) {
	m := doc.Metrics()

	observe := func(category string, vs []int) {
		o := metrics.InputSize.WithLabelValues(category)
		for _, v := range vs {
			o.Observe(float64(v))
		}
	}

	observe("tex", m.TexFiles)
	observe("asset", m.AssetFiles)
	observe("data", m.DataFiles)
	observe("other", m.OtherFiles)

	if m.Result >= 0 {
		metrics.OutputSize.Observe(float64(m.Result))
	}
}

package service

import (
	"encoding/json"
	"net/http"

	"github.com/digineo/texd"
	"github.com/digineo/texd/tex"
)

type Status struct {
	Version       string   `json:"version"`
	Mode          string   `json:"mode"`
	Images        []string `json:"images,omitempty"`
	Timeout       float64  `json:"timeout"` // job timeout in seconds
	Engines       []string `json:"engines"`
	DefaultEngine string   `json:"default_engine"`
	Queue         struct {
		Length   int `json:"length"`
		Capacity int `json:"capacity"`
	} `json:"queue"`
}

func (svc *service) HandleStatus(res http.ResponseWriter, req *http.Request) {
	status := Status{
		Version:       texd.Version(),
		Mode:          svc.mode,
		Images:        svc.images,
		Timeout:       svc.compileTimeout.Seconds(),
		Engines:       tex.SupportedEngines(),
		DefaultEngine: tex.DefaultEngine.Name(),
	}
	status.Queue.Length = len(svc.jobs)
	status.Queue.Capacity = cap(svc.jobs)

	res.Header().Set("Content-Type", mimeTypeJSON)
	res.Header().Set("X-Content-Type-Options", "nosniff")
	res.WriteHeader(http.StatusOK)
	json.NewEncoder(res).Encode(&status)
}

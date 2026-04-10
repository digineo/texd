package main

import (
	"runtime"
	"time"

	"github.com/digineo/texd/service"
	"github.com/digineo/texd/tex"
	"github.com/docker/go-units"
	"go.uber.org/zap/zapcore"
)

const (
	defaultQueueTimeout       = 10 * time.Second
	defaultMaxJobSize         = 50 * units.MiB
	defaultCompileTimeout     = time.Minute
	defaultRetentionPoolSize  = 100 * units.MiB
	defaultRetentionPoolItems = 1000
)

// config holds all command-line configuration.
type config struct {
	// Server options
	addr           string
	queueLength    int
	queueTimeout   time.Duration
	maxJobSize     string // human-readable size
	compileTimeout time.Duration

	// TeX options
	engine      string
	shellEscape int // 0=default, 1=enable, -1=disable
	jobDir      string
	keepJobs    int

	// Docker options
	pull   bool
	images []string // remaining args after flag parsing

	// Reference store
	storageDSN  string
	retPolicy   int
	retPolItems int
	retPolSize  string // human-readable size

	// Misc
	logLevel    string
	showVersion bool
}

// defaultConfig returns a config with default values.
func defaultConfig() *config {
	return &config{
		addr:           ":2201",
		queueLength:    runtime.GOMAXPROCS(0),
		queueTimeout:   defaultQueueTimeout,
		maxJobSize:     units.BytesSize(float64(defaultMaxJobSize)),
		compileTimeout: defaultCompileTimeout,
		engine:         tex.DefaultEngine.Name(),
		shellEscape:    0,
		jobDir:         "",
		keepJobs:       service.KeepJobsNever,
		pull:           false,
		images:         nil,
		storageDSN:     "",
		retPolicy:      0,
		retPolItems:    defaultRetentionPoolItems,
		retPolSize:     units.BytesSize(float64(defaultRetentionPoolSize)),
		logLevel:       zapcore.InfoLevel.String(),
		showVersion:    false,
	}
}

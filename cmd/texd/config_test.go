package main

import (
	"testing"

	"github.com/digineo/texd/service"
	"github.com/digineo/texd/tex"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	assert.Equal(t, ":2201", cfg.addr)
	assert.Greater(t, cfg.queueLength, 0)
	assert.Equal(t, defaultQueueTimeout, cfg.queueTimeout)
	assert.Equal(t, defaultCompileTimeout, cfg.compileTimeout)
	assert.Equal(t, tex.DefaultEngine.Name(), cfg.engine)
	assert.Equal(t, 0, cfg.shellEscape)
	assert.Equal(t, "", cfg.jobDir)
	assert.Equal(t, service.KeepJobsNever, cfg.keepJobs)
	assert.False(t, cfg.pull)
	assert.Nil(t, cfg.images)
	assert.Equal(t, "", cfg.storageDSN)
	assert.Equal(t, 0, cfg.retPolicy)
	assert.Equal(t, 1000, cfg.retPolItems)
	assert.Equal(t, zapcore.InfoLevel.String(), cfg.logLevel)
	assert.False(t, cfg.showVersion)
}

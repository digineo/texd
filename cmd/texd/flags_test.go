package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/digineo/texd/service"
	"github.com/digineo/texd/tex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlags(t *testing.T) { //nolint: funlen
	tests := []struct {
		name        string
		args        []string
		want        func(*config)
		wantErr     error
		errContains string
	}{
		{
			name: "default values",
			args: []string{},
			want: func(cfg *config) {
				assert.Equal(t, ":2201", cfg.addr)
				assert.Equal(t, tex.DefaultEngine.Name(), cfg.engine)
			},
		},
		{
			name:    "help flag",
			args:    []string{"--help"},
			want:    nil,
			wantErr: errHelpRequested,
		},
		{
			name:    "version flag",
			args:    []string{"--version"},
			want:    nil,
			wantErr: errVersionRequested,
		},
		{
			name:    "version flag short",
			args:    []string{"-v"},
			want:    nil,
			wantErr: errVersionRequested,
		},
		{
			name: "listen address",
			args: []string{"-b", ":8080"},
			want: func(cfg *config) {
				assert.Equal(t, ":8080", cfg.addr)
			},
		},
		{
			name: "tex engine",
			args: []string{"-X", "xelatex"},
			want: func(cfg *config) {
				assert.Equal(t, "xelatex", cfg.engine)
			},
		},
		{
			name: "shell escape enabled",
			args: []string{"--shell-escape"},
			want: func(cfg *config) {
				assert.Equal(t, 1, cfg.shellEscape)
			},
		},
		{
			name: "shell escape disabled",
			args: []string{"--no-shell-escape"},
			want: func(cfg *config) {
				assert.Equal(t, -1, cfg.shellEscape)
			},
		},
		{
			name:        "shell escape mutually exclusive",
			args:        []string{"--shell-escape", "--no-shell-escape"},
			want:        nil,
			errContains: "mutually exclusive",
		},
		{
			name: "compile timeout",
			args: []string{"-t", "2m"},
			want: func(cfg *config) {
				assert.Equal(t, 2*time.Minute, cfg.compileTimeout)
			},
		},
		{
			name: "parallel jobs",
			args: []string{"-P", "8"},
			want: func(cfg *config) {
				assert.Equal(t, 8, cfg.queueLength)
			},
		},
		{
			name: "max job size",
			args: []string{"--max-job-size", "100MB"},
			want: func(cfg *config) {
				assert.Equal(t, "100MB", cfg.maxJobSize)
			},
		},
		{
			name: "queue wait",
			args: []string{"-w", "30s"},
			want: func(cfg *config) {
				assert.Equal(t, 30*time.Second, cfg.queueTimeout)
			},
		},
		{
			name: "job directory",
			args: []string{"-D", "/tmp/jobs"},
			want: func(cfg *config) {
				assert.Equal(t, "/tmp/jobs", cfg.jobDir)
			},
		},
		{
			name: "reference store",
			args: []string{"--reference-store", "dir:///tmp/store"},
			want: func(cfg *config) {
				assert.Equal(t, "dir:///tmp/store", cfg.storageDSN)
			},
		},
		{
			name: "pull flag",
			args: []string{"--pull"},
			want: func(cfg *config) {
				assert.True(t, cfg.pull)
			},
		},
		{
			name: "log level",
			args: []string{"--log-level", "debug"},
			want: func(cfg *config) {
				assert.Equal(t, "debug", cfg.logLevel)
			},
		},
		{
			name: "keep jobs never",
			args: []string{"--keep-jobs", "never"},
			want: func(cfg *config) {
				assert.Equal(t, service.KeepJobsNever, cfg.keepJobs)
			},
		},
		{
			name: "keep jobs always",
			args: []string{"--keep-jobs", "always"},
			want: func(cfg *config) {
				assert.Equal(t, service.KeepJobsAlways, cfg.keepJobs)
			},
		},
		{
			name: "keep jobs on-failure",
			args: []string{"--keep-jobs", "on-failure"},
			want: func(cfg *config) {
				assert.Equal(t, service.KeepJobsOnFailure, cfg.keepJobs)
			},
		},
		{
			name: "retention policy keep",
			args: []string{"-R", "keep"},
			want: func(cfg *config) {
				assert.Equal(t, 0, cfg.retPolicy)
			},
		},
		{
			name: "retention policy purge",
			args: []string{"-R", "purge-on-start"},
			want: func(cfg *config) {
				assert.Equal(t, 1, cfg.retPolicy)
			},
		},
		{
			name: "retention policy access",
			args: []string{"-R", "access"},
			want: func(cfg *config) {
				assert.Equal(t, 2, cfg.retPolicy)
			},
		},
		{
			name: "retention policy access items",
			args: []string{"--rp-access-items", "500"},
			want: func(cfg *config) {
				assert.Equal(t, 500, cfg.retPolItems)
			},
		},
		{
			name: "retention policy access size",
			args: []string{"--rp-access-size", "200MB"},
			want: func(cfg *config) {
				assert.Equal(t, "200MB", cfg.retPolSize)
			},
		},
		{
			name: "docker images",
			args: []string{"texlive/texlive:latest"},
			want: func(cfg *config) {
				assert.Equal(t, []string{"texlive/texlive:latest"}, cfg.images)
			},
		},
		{
			name: "multiple docker images",
			args: []string{"image1:latest", "image2:v1.0"},
			want: func(cfg *config) {
				assert.Equal(t, []string{"image1:latest", "image2:v1.0"}, cfg.images)
			},
		},
		{
			name: "combined flags",
			args: []string{
				"-b", "localhost:9000",
				"-X", "lualatex",
				"-t", "5m",
				"-P", "4",
				"--log-level", "warn",
			},
			want: func(cfg *config) {
				assert.Equal(t, "localhost:9000", cfg.addr)
				assert.Equal(t, "lualatex", cfg.engine)
				assert.Equal(t, 5*time.Minute, cfg.compileTimeout)
				assert.Equal(t, 4, cfg.queueLength)
				assert.Equal(t, "warn", cfg.logLevel)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			cfg, err := parseFlags("texd", tt.args, stderr)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, cfg)
				return
			}

			if tt.errContains != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, cfg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.want != nil {
				tt.want(cfg)
			}
		})
	}
}

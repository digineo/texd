package main

import (
	"testing"
	"time"

	"github.com/digineo/texd/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRetentionPolicy(t *testing.T) { //nolint: funlen
	tests := []struct {
		name     string
		policy   int
		items    int
		size     string
		wantErr  bool
		wantType string
	}{
		{
			name:     "keep forever policy",
			policy:   0,
			items:    0,
			size:     "0",
			wantErr:  false,
			wantType: "*refstore.KeepForever",
		},
		{
			name:     "purge on start policy",
			policy:   1,
			items:    0,
			size:     "0",
			wantErr:  false,
			wantType: "*refstore.PurgeOnStart",
		},
		{
			name:     "access list policy",
			policy:   2,
			items:    100,
			size:     "50MB",
			wantErr:  false,
			wantType: "*refstore.AccessList",
		},
		{
			name:     "access list with large size",
			policy:   2,
			items:    1000,
			size:     "1GB",
			wantErr:  false,
			wantType: "*refstore.AccessList",
		},
		{
			name:    "access list with invalid size",
			policy:  2,
			items:   100,
			size:    "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pol, err := createRetentionPolicy(tt.policy, tt.items, tt.size)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, pol)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, pol)

			// Verify the type
			assert.Contains(t, tt.wantType, "*refstore.")
		})
	}
}

func TestConfigureTeX(t *testing.T) { //nolint: funlen
	// Setup logger for testing
	log, sync, err := setupLogger("error", true)
	require.NoError(t, err)
	defer sync()

	tests := []struct {
		name    string
		cfg     *config
		wantErr bool
	}{
		{
			name: "default engine",
			cfg: &config{
				engine:      "pdflatex",
				shellEscape: 0,
				jobDir:      "",
			},
			wantErr: false,
		},
		{
			name: "xelatex engine",
			cfg: &config{
				engine:      "xelatex",
				shellEscape: 0,
				jobDir:      "",
			},
			wantErr: false,
		},
		{
			name: "lualatex engine",
			cfg: &config{
				engine:      "lualatex",
				shellEscape: 0,
				jobDir:      "",
			},
			wantErr: false,
		},
		{
			name: "shell escape enabled",
			cfg: &config{
				engine:      "pdflatex",
				shellEscape: 1,
				jobDir:      "",
			},
			wantErr: false,
		},
		{
			name: "shell escape disabled",
			cfg: &config{
				engine:      "pdflatex",
				shellEscape: -1,
				jobDir:      "",
			},
			wantErr: false,
		},
		{
			name: "invalid engine",
			cfg: &config{
				engine:      "invalidengine",
				shellEscape: 0,
				jobDir:      "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := configureTeX(tt.cfg, log)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestBuildServiceOptions(t *testing.T) { //nolint: funlen
	// Setup logger for testing
	log, sync, err := setupLogger("error", true)
	require.NoError(t, err)
	defer sync()

	tests := []struct {
		name    string
		cfg     *config
		wantErr bool
		check   func(*testing.T, service.Options)
	}{
		{
			name: "minimal config",
			cfg: &config{
				addr:           ":2201",
				queueLength:    4,
				queueTimeout:   10 * time.Second,
				maxJobSize:     "50MB",
				compileTimeout: time.Minute,
				keepJobs:       service.KeepJobsNever,
				storageDSN:     "",
				images:         nil,
			},
			wantErr: false,
			check: func(t *testing.T, opts service.Options) {
				assert.Equal(t, ":2201", opts.Addr)
				assert.Equal(t, 4, opts.QueueLength)
				assert.Equal(t, 10*time.Second, opts.QueueTimeout)
				assert.Equal(t, time.Minute, opts.CompileTimeout)
				assert.Equal(t, "local", opts.Mode)
				assert.NotNil(t, opts.RefStore)
			},
		},
		{
			name: "invalid max job size",
			cfg: &config{
				addr:           ":2201",
				queueLength:    4,
				queueTimeout:   10 * time.Second,
				maxJobSize:     "invalid",
				compileTimeout: time.Minute,
				keepJobs:       service.KeepJobsNever,
				storageDSN:     "",
				images:         nil,
			},
			wantErr: true,
		},
		{
			name: "invalid retention policy size",
			cfg: &config{
				addr:           ":2201",
				queueLength:    4,
				queueTimeout:   10 * time.Second,
				maxJobSize:     "50MB",
				compileTimeout: time.Minute,
				keepJobs:       service.KeepJobsNever,
				storageDSN:     "memory://",
				retPolicy:      2,
				retPolItems:    1000,
				retPolSize:     "invalid",
				images:         nil,
			},
			wantErr: true,
		},
		{
			name: "invalid storage DSN",
			cfg: &config{
				addr:           ":2201",
				queueLength:    4,
				queueTimeout:   10 * time.Second,
				maxJobSize:     "50MB",
				compileTimeout: time.Minute,
				keepJobs:       service.KeepJobsNever,
				storageDSN:     "invalid://bad",
				retPolicy:      0,
				retPolItems:    1000,
				retPolSize:     "100MB",
				images:         nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := buildServiceOptions(tt.cfg, log)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, opts)
			}
		})
	}
}

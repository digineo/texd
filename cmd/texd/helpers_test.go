package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintVersion(t *testing.T) {
	buf := &bytes.Buffer{}
	printVersion(buf)

	output := buf.String()

	assert.Contains(t, output, "Go:")
	assert.Contains(t, output, "Dependencies:")
	assert.Contains(t, output, "github.com/digineo/texd")
}

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		level       string
		development bool
		expectError bool
	}{
		{
			level:       "info",
			development: false,
		},
		{
			level:       "debug",
			development: true,
		},
		{
			level:       "warn",
			development: false,
		},
		{
			level:       "error",
			development: false,
		},
		{
			level:       "invalid",
			development: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			log, sync, err := setupLogger(tt.level, tt.development)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, log)
			require.NotNil(t, sync)

			// Verify we can use the logger
			log.Info("test message")

			// Call sync cleanup
			sync()
		})
	}
}

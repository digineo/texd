package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun_HelpFlag(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode, err := run([]string{"texd", "--help"}, stdout, stderr)

	assert.Equal(t, exitSuccess, exitCode)
	assert.ErrorIs(t, err, errHelpRequested)
	assert.Contains(t, stdout.String(), "texd") // banner should be printed
}

func TestRun_VersionFlag(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode, err := run([]string{"texd", "--version"}, stdout, stderr)

	assert.Equal(t, exitSuccess, exitCode)
	assert.ErrorIs(t, err, errVersionRequested)
	assert.Contains(t, stdout.String(), "texd") // banner should be printed
	assert.Contains(t, stdout.String(), "Go:")  // version info should be printed
}

func TestRun_InvalidFlag(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode, err := run([]string{"texd", "--invalid-flag"}, stdout, stderr)

	assert.Equal(t, exitFlagErr, exitCode)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, errHelpRequested)
	assert.NotErrorIs(t, err, errVersionRequested)
}

func TestRun_InvalidEngine(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode, err := run([]string{"texd", "-X", "invalidengine"}, stdout, stderr)

	assert.Equal(t, exitFlagErr, exitCode)
	assert.Error(t, err)
}

func TestRun_MutuallyExclusiveFlags(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode, err := run([]string{"texd", "--shell-escape", "--no-shell-escape"}, stdout, stderr)

	assert.Equal(t, exitFlagErr, exitCode)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

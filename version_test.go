package texd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionData_Version(t *testing.T) {
	subject := versionData{version: "1.0", isdev: "0"}
	assert.Equal(t, "1.0 (release)", subject.Version())

	subject.isdev = "1"
	assert.Equal(t, "1.0 (development)", subject.Version())
}

func TestVersionData_PrintBanner(t *testing.T) {
	subject := versionData{
		version:  "0.99-42-dirty",
		isdev:    "1",
		commitat: "2022-04-01T12:34:56Z",
		buildat:  "2022-04-02T12:34:56Z",
		commit:   "c0ffee",
	}

	const expected = `
_____    _   _ ____     texd version 0.99-42-dirty (development)
  | ____  \_/  |   \    commit       c0ffee
  | |___ _/ \_ |___/    commit date  2022-04-01T12:34:56Z
    |___                build date   2022-04-02T12:34:56Z

`

	var actual bytes.Buffer
	subject.PrintBanner(&actual)
	assert.Equal(t, expected, actual.String())
}

func TestGloabls(t *testing.T) {
	assert.True(t, Development())
	assert.Equal(t, "development (development)", Version())

	const expected = `
_____    _   _ ____     texd version development (development)
  | ____  \_/  |   \    commit       HEAD
  | |___ _/ \_ |___/    commit date  unknown
    |___                build date   unknown

`

	var actual bytes.Buffer
	PrintBanner(&actual)
	assert.Equal(t, expected, actual.String())
}

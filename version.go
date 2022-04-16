package texd

import (
	"fmt"
	"io"
)

// Versioning variables. These are set at compile time through -ldflags.
var (
	version  = "development"
	commit   = "HEAD"
	commitat = "unknown"
	buildat  = "unknown"
	isdev    = "1"
)

const banner = `
_____    _   _ ____     texd version %s
  | ____  \_/  |   \    commit       %s
  | |___ _/ \_ |___/    commit date  %s
    |___                build date   %s

`

type versionData struct{ version, commit, commitat, buildat, isdev string }

var v = versionData{version, commit, commitat, buildat, isdev}

func (d *versionData) Version() string {
	if d.Development() {
		return d.version + " (development)"
	}
	return d.version + " (release)"
}

func (d *versionData) Development() bool {
	return d.isdev == "1"
}

func (d *versionData) PrintBanner(w io.Writer) {
	fmt.Fprintf(w, banner, d.Version(), d.commit, d.commitat, d.buildat)
}

// Version returns a string describing the version. For release versions
// this will contain the Git tag and commit ID. When used as library (or
// in development), this may return just "development".
func Version() string {
	return v.Version()
}

func Development() bool {
	return v.Development()
}

// PrintBanner will write a small ASCII graphic and versioning
// information to w.
func PrintBanner(w io.Writer) {
	v.PrintBanner(w)
}

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
)

const banner = `
_____    _   _ ____     texd version %s
  | ____  \_/  |   \    commit       %s
  | |___ _/ \_ |___/    commit date  %s
    |___                build date   %s

`

// Version returns a string describing the version. For release versions
// this will contain the Git tag and commit ID. When used as library (or
// in development), this may return just "development".
func Version() string {
	return version
}

// PrintBanner will write a small ASCII graphic and versioning
// information to w.
func PrintBanner(w io.Writer) {
	fmt.Fprintf(w, banner, version, commit, commitat, buildat)
}

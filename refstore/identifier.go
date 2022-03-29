package refstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

type ErrInvalidIdentifier struct {
	msg   string
	cause error
}

func (err *ErrInvalidIdentifier) Error() string {
	if err.cause == nil {
		return fmt.Sprintf("invalid identifier: %v", err.msg)
	}
	return fmt.Sprintf("invalid identifier: %s: %v", err.msg, err.cause)
}

func (err *ErrInvalidIdentifier) Unwrap() error {
	return err.cause
}

var (
	prefix = []byte("sha256:")
	plen   = len(prefix)
	stdlen = base64.StdEncoding.EncodedLen(sha256.Size)    // 44 bytes, incl. 1 byte padding
	rawlen = base64.RawStdEncoding.EncodedLen(sha256.Size) // 43 bytes
)

// File references are identified by their checksum.
type Identifier string

func (id Identifier) String() string {
	return "sha256:" + string(id)
}

func (id Identifier) Raw() string {
	return string(id)
}

// ToIdentifier converts b into the canonical Identifier representation.
// This returns an error if len(b) is not sha256.Size (32).
func ToIdentifier(b []byte) (Identifier, error) {
	if len(b) != sha256.Size {
		return "", &ErrInvalidIdentifier{msg: "unexpected length"}
	}
	id := base64.RawURLEncoding.EncodeToString(b)
	return Identifier(id), nil
}

// ParseIdentifier takes an input in the form "sha256:...." and transforms it
// into an Identifier. Parsing errors are reported as ErrInvalidIdentifier.
func ParseIdentifier(b []byte) (Identifier, error) {
	n := len(b)
	if n != rawlen+plen && n != stdlen+plen {
		return "", &ErrInvalidIdentifier{msg: "unexpected input length"}
	}
	if !bytes.HasPrefix(b, prefix) {
		return "", &ErrInvalidIdentifier{msg: "missing hash prefix"}
	}
	if n == stdlen+plen && b[n-1] != byte(base64.StdPadding) {
		return "", &ErrInvalidIdentifier{msg: "unexpected non-padding character at the end"}
	}

	b = b[plen:] // remove prefix and padding
	if i := bytes.IndexRune(b, base64.StdPadding); i >= 0 {
		b = b[:i] // remove padding character(s)
	}

	dec := base64.RawURLEncoding
	if bytes.ContainsAny(b, "+/") {
		dec = base64.RawStdEncoding
	}

	b, err := dec.DecodeString(string(b))
	if err != nil {
		return "", &ErrInvalidIdentifier{"decoding failed", err}
	}
	return ToIdentifier(b)
}

// NewIdentifier calculates the reference ID from the given file contents.
func NewIdentifier(contents []byte) Identifier {
	h := sha256.Sum256(contents)
	id, _ := ToIdentifier(h[:])
	return id
}

// ReadIdentifier creates an identifier of the contents read from r.
func ReadIdentifier(r io.Reader) (Identifier, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	id, _ := ToIdentifier(h.Sum(nil))
	return id, nil
}

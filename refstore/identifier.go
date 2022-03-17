package refstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
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

// File references are identified by their checksum.
type Identifier []byte

var (
	prefix = []byte("sha256:")
	plen   = len(prefix)
	stdlen = base64.StdEncoding.EncodedLen(sha256.Size)    // 44 bytes, incl. 1 byte padding
	rawlen = base64.RawStdEncoding.EncodedLen(sha256.Size) // 43 bytes
)

func (id Identifier) String() string {
	return "sha256:" + id.Raw()
}

func (id Identifier) Raw() string {
	return base64.RawURLEncoding.EncodeToString(id)
}

// ParseIdentifier takes an input in the form "sha256:...." and transforms it
// into an Identifier. Parsing errors are reported as ErrInvalidIdentifier.
func ParseIdentifier(b []byte) (Identifier, error) {
	n := len(b)
	if n != rawlen+plen && n != stdlen+plen {
		return nil, &ErrInvalidIdentifier{msg: "unexpected input length"}
	}
	if !bytes.HasPrefix(b, prefix) {
		return nil, &ErrInvalidIdentifier{msg: "missing hash prefix"}
	}
	if n == stdlen+plen && b[n-1] != byte(base64.StdPadding) {
		return nil, &ErrInvalidIdentifier{msg: "unexpected non-padding character at the end"}
	}

	// remove prefix and padding
	b = b[len(prefix):]

	// remove padding character(s)
	if i := bytes.IndexRune(b, base64.StdPadding); i >= 0 {
		b = b[:i]
	}

	dec := base64.RawURLEncoding
	if bytes.ContainsAny(b, "+/") {
		dec = base64.RawStdEncoding
	}

	id, err := dec.DecodeString(string(b))
	if err != nil {
		return nil, &ErrInvalidIdentifier{"decoding failed", err}
	} else if len(id) != sha256.Size {
		log.Println(len(id), sha256.Size)
		return nil, &ErrInvalidIdentifier{msg: "decoding failed: unexpected output length"}
	}
	return id, nil
}

// NewIdentifier calculates the reference ID from the given file contents.
func NewIdentifier(contents []byte) Identifier {
	h := sha256.Sum256(contents)
	return Identifier(h[:])
}

func ReadIdentifier(r io.Reader) (Identifier, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil, err
	}
	id := h.Sum(nil)
	return Identifier(id), nil
}

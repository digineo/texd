package refstore

import (
	"errors"
	"io"

	"go.uber.org/zap"
)

// The Adapter inteface describes the protocol to interact with different
// storage backends.
type Adapter interface {
	// CopyFile copies a file with the given ID to the target path. It may
	// return ErrUnknownReference if the ID is unknown.
	CopyFile(log *zap.Logger, id Identifier, w io.Writer) error

	// Store saves the content in the adapter backend.
	Store(log *zap.Logger, contents []byte) error
}

var ErrUnknownReference = errors.New("unknown reference")

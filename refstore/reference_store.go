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
	Store(log *zap.Logger, r io.Reader) error

	// Exists checks whether the given reference identifier exists in this
	// storage adapter.
	Exists(id Identifier) bool
}

// ErrUnknownReference can be returned from Adapter implementations, if
// a given Identifier is unknown to them.
var ErrUnknownReference = errors.New("unknown reference")

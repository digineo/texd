// Package nop implements a no-op reference store adapter.
// It does nothing, and can't be configured.
package nop

import (
	"io"
	"net/url"

	"github.com/digineo/texd/refstore"
	"go.uber.org/zap"
)

type nop struct{}

func New(_ *url.URL, _ refstore.RetentionPolicy) (refstore.Adapter, error) {
	return &nop{}, nil
}

func (*nop) CopyFile(*zap.Logger, refstore.Identifier, io.Writer) error {
	return refstore.ErrUnknownReference
}

func (*nop) Store(_ *zap.Logger, r io.Reader) error {
	_, err := io.Copy(io.Discard, r)
	return err
}

func (*nop) Exists(refstore.Identifier) bool {
	return false
}

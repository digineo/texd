package exec

import (
	"context"

	"github.com/dmke/texd/tex"
)

type localExec struct {
	baseExec
}

func NewLocalExec(doc tex.Document) Exec {
	return &localExec{
		baseExec: baseExec{doc: doc},
	}
}

func (x *localExec) Run(ctx context.Context) (Result, error) {
	return nil, nil
}

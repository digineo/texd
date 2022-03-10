package exec

import (
	"context"

	"github.com/dmke/texd/tex"
)

type dockerExec struct {
	baseExec
}

func NewDockerExec(doc tex.Document) Exec {
	return &dockerExec{
		baseExec: baseExec{doc: doc},
	}
}

func (x *dockerExec) Run(ctx context.Context) error {
	return nil
}

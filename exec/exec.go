package exec

import (
	"context"

	"github.com/dmke/texd/tex"
)

type Exec interface {
	Run(context.Context) (Result, error)
}

type Result interface {
	Success() bool
	Cleanup() error
}

type baseExec struct {
	doc tex.Document
	cmd []string
}

func (x *baseExec) extract() (dir string, cmd []string, err error) {
	flags, err := x.doc.Engine().CmdFlags()
	if err != nil {
		return "", nil, err
	}
	main, err := x.doc.MainInput()
	if err != nil {
		return "", nil, err
	}

	cmd = []string{"latexmk"}
	cmd = append(cmd, flags...)
	cmd = append(cmd, main)
	dir, err = x.doc.WorkingDirectory()
	return
}

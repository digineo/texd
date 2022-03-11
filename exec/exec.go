package exec

import (
	"context"

	"github.com/dmke/texd/tex"
)

type Exec interface {
	Run(context.Context) error
}

type baseExec struct {
	doc tex.Document
	cmd []string
}

func (x *baseExec) extract() (dir string, cmd []string, err error) {
	main, err := x.doc.MainInput()
	if err != nil {
		return "", nil, err
	}

	cmd = x.doc.Engine().LatexmkCmd(main)
	dir, err = x.doc.WorkingDirectory()
	return
}

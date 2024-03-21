package exec

import (
	"context"

	"github.com/digineo/texd/tex"
	"go.uber.org/zap"
)

type Exec interface {
	Run(ctx context.Context, logger *zap.Logger) error
}

// Document is a sub-set of the tex.Document interface.
type Document interface {
	WorkingDirectory() (string, error)
	MainInput() (string, error)
	Engine() tex.Engine
	Image() string
}

var _ Document = (tex.Document)(nil)

type baseExec struct {
	doc Document
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

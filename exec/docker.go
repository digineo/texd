package exec

import (
	"context"

	"github.com/digineo/texd/tex"
	"github.com/digineo/texd/xlog"
)

type dockerRunner interface {
	Run(ctx context.Context, tag, wd string, cmd []string) (string, error)
}

type dockerExec struct {
	cli dockerRunner
	baseExec
}

func (dc *DockerClient) Executor(doc Document) Exec {
	return &dockerExec{
		baseExec: baseExec{doc: doc},
		cli:      dc,
	}
}

func (x *dockerExec) Run(ctx context.Context, log xlog.Logger) error {
	dir, cmd, err := x.extract()
	if err != nil {
		return tex.CompilationError("invalid document", err, nil)
	}

	tag := x.doc.Image()
	log.Debug("running latexmk", xlog.Any("args", cmd[1:]))
	output, err := x.cli.Run(ctx, tag, dir, cmd)
	if err != nil {
		log.Error("compilation failed", xlog.Error(err))
		return tex.CompilationError("compilation failed", err, tex.KV{
			"cmd":    cmd[0],
			"args":   cmd[1:],
			"output": output,
			"image":  tag,
		})
	}
	return nil
}

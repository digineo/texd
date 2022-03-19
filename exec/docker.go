package exec

import (
	"context"

	"github.com/digineo/texd/tex"
	"go.uber.org/zap"
)

type dockerExec struct {
	cli *DockerClient
	baseExec
}

func (dc *DockerClient) Executor(doc Document) Exec {
	return &dockerExec{
		baseExec: baseExec{doc: doc},
		cli:      dc,
	}
}

func (x *dockerExec) Run(ctx context.Context, log *zap.Logger) error {
	dir, cmd, err := x.extract()
	if err != nil {
		return tex.CompilationError("invalid document", err, nil)
	}

	tag := x.doc.Image()
	log.Debug("running latexmk", zap.Strings("args", cmd[1:]))
	output, err := x.cli.Run(ctx, tag, dir, cmd)
	if err != nil {
		log.Error("compilation failed", zap.Error(err))
		return tex.CompilationError("compilation failed", err, tex.KV{
			"cmd":    cmd[0],
			"args":   cmd[1:],
			"output": output,
			"image":  tag,
		})
	}
	return nil
}

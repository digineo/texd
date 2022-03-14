package exec

import (
	"context"

	"github.com/digineo/texd/tex"
)

type dockerExec struct {
	cli *DockerClient
	baseExec
}

func (dc *DockerClient) Executor(doc tex.Document) Exec {
	return &dockerExec{
		baseExec: baseExec{doc: doc},
		cli:      dc,
	}
}

func (x *dockerExec) Run(ctx context.Context) error {
	dir, cmd, err := x.extract()
	if err != nil {
		return tex.CompilationError("invalid document", err, nil)
	}

	tag := x.doc.Image()
	output, err := x.cli.Run(ctx, tag, dir, cmd)
	if err != nil {
		return tex.CompilationError("compilation failed", err, tex.KV{
			"flags":  cmd,
			"output": output,
			"image":  tag,
		})
	}
	return nil
}

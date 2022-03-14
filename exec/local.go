package exec

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/digineo/texd/tex"
)

type localExec struct {
	baseExec
}

func LocalExec(doc tex.Document) Exec {
	return &localExec{
		baseExec: baseExec{doc: doc},
	}
}

func (x *localExec) Run(ctx context.Context) error {
	dir, args, err := x.extract()
	if err != nil {
		return tex.CompilationError("invalid document", err, nil)
	}

	if len(args) < 2 {
		return tex.UnknownError("unexpected command: too few arguments", nil, tex.KV{
			"args": args,
		})
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return tex.CompilationError("compilation failed", err, tex.KV{
			"cmd":    args[0],
			"args":   args[1:],
			"output": stderr.String(),
		})
	}
	return nil
}

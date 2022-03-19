package exec

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/digineo/texd/tex"
	"go.uber.org/zap"
)

type localExec struct {
	baseExec
}

func LocalExec(doc Document) Exec {
	return &localExec{
		baseExec: baseExec{doc: doc},
	}
}

func (x *localExec) Run(ctx context.Context, log *zap.Logger) error {
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

	log.Debug("running latexmk", zap.Strings("args", args[1:]))
	if err := cmd.Run(); err != nil {
		log.Error("compilation failed", zap.Error(err))
		return tex.CompilationError("compilation failed", err, tex.KV{
			"cmd":    args[0],
			"args":   args[1:],
			"output": stderr.String(),
		})
	}
	return nil
}

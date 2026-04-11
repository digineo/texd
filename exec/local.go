package exec

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/digineo/texd/tex"
	"github.com/digineo/texd/xlog"
)

type localExec struct {
	baseExec
	path string // overrides command to execute (in tests)
}

func LocalExec(doc Document) Exec {
	return &localExec{
		baseExec: baseExec{doc: doc},
	}
}

func (x *localExec) Run(ctx context.Context, log xlog.Logger) error {
	dir, args, err := x.extract()
	if err != nil {
		return tex.CompilationError("invalid document", err, nil)
	}

	if x.path != "" {
		// in tests, we don't want to actually execute latexmk
		args[0] = x.path
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stderr = &stderr

	log.Debug("running latexmk", xlog.Any("args", args[1:]))
	if err := cmd.Run(); err != nil {
		log.Error("compilation failed",
			xlog.String("stderr", stderr.String()),
			xlog.Error(err))
		return tex.CompilationError("compilation failed", err, tex.KV{
			"cmd":    args[0],
			"args":   args[1:],
			"output": stderr.String(),
		})
	}
	return nil
}

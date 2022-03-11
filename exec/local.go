package exec

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/dmke/texd/tex"
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
	dir, flags, err := x.extract()
	if err != nil {
		return tex.CompilationError("invalid document", err, nil)
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "latexmk", flags...) // #nosec
	cmd.Dir = dir
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return tex.CompilationError("compilation failed", err, tex.KV{
			"flags":  flags,
			"stderr": stderr.String(),
		})
	}
	return nil
}

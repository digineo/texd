package exec

import (
	"context"
	"strings"

	"github.com/digineo/texd/tex"
	"go.uber.org/zap"
)

type MockExec struct {
	baseExec

	// ShouldFail controls whether Run will return an error or not.
	ShouldFail bool

	// Result holds content written to either the document's result PDF
	// file, or its correspondint error log file, depending on the
	// value of ShouldFail.
	ResultContents string
}

// Mock can be used in tests to avoid executing real commands.
//
//	service.Executor := exec.Mock(true, pdfFixture)
//
// Imprtant note: Mock will panic if the document's main input file
// does not contain a dot + file extension, of if the result content
// can't be written.
func Mock(shouldFail bool, resultContents string) func(tex.Document) Exec {
	return func(doc tex.Document) Exec {
		return &MockExec{
			baseExec:       baseExec{doc: doc},
			ShouldFail:     shouldFail,
			ResultContents: resultContents,
		}
	}
}

func (x *MockExec) Run(ctx context.Context, log *zap.Logger) error {
	_, args, err := x.extract()
	if err != nil {
		return tex.CompilationError("invalid document", err, nil)
	}

	if len(args) < 2 {
		return tex.UnknownError("unexpected command: too few arguments", nil, tex.KV{
			"args": args,
		})
	}

	log.Debug("simlate running latexmk", zap.Strings("args", args[1:]))
	main, _ := x.doc.MainInput() // would have failed in x.extract()
	dot := strings.LastIndexByte(main, '.')
	if dot < 0 {
		panic("malformed input file: missing extension")
	}
	var outfile string
	if x.ShouldFail {
		outfile = main[:dot] + ".log"
	} else {
		outfile = main[:dot] + ".pdf"
	}

	if err := x.doc.AddFile(outfile, x.ResultContents); err != nil {
		panic("failed to store result file")
	}

	if x.ShouldFail {
		log.Error("compilation failed", zap.Error(err))
		return tex.CompilationError("compilation failed", err, tex.KV{
			"cmd":  args[0],
			"args": args[1:],
		})
	}
	return nil
}

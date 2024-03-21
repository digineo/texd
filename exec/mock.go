package exec

import (
	"context"
	"fmt"
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
// Important note: Mock will panic if the document's main input file
// does not contain a dot + file extension, of if the result content
// can't be written.
func Mock(shouldFail bool, resultContents string) func(Document) Exec {
	return func(doc Document) Exec {
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

	adder, ok := x.doc.(interface {
		AddFile(name string, content string) error
	})
	if !ok {
		panic("can't add files to document")
	}
	if err := adder.AddFile(outfile, x.ResultContents); err != nil {
		panic(fmt.Errorf("failed to store result file: %w", err))
	}

	if x.ShouldFail {
		return tex.CompilationError("compilation failed", nil, tex.KV{
			"cmd":  args[0],
			"args": args[1:],
		})
	}
	return nil
}

package tex

import "fmt"

type Engine string

const (
	PDFLaTeX = Engine("pdflatex")
	XeLaTeX  = Engine("xelatex")
	LuaLaTeX = Engine("lualatex")
)

func SupportedEngines() []Engine {
	return []Engine{PDFLaTeX, XeLaTeX, LuaLaTeX}
}

func ParseTeXEngine(s string) (Engine, error) {
	switch x := Engine(s); x {
	case PDFLaTeX, XeLaTeX, LuaLaTeX:
		return x, nil
	default:
		return "", ErrUnsupportedEngine(x)
	}
}

var DefaultEngine = XeLaTeX

func (x Engine) CmdFlags() ([]string, error) {
	switch x {
	case XeLaTeX:
		return []string{"-pdfxe"}, nil
	case LuaLaTeX:
		return []string{"-pdflua"}, nil
	case PDFLaTeX:
		return []string{"-pdf"}, nil
	default:
		return nil, ErrUnsupportedEngine(x)
	}
}

func (x Engine) String() string {
	return string(x)
}

type ErrUnsupportedEngine Engine

func (err ErrUnsupportedEngine) Error() string {
	return fmt.Sprintf("unsupported TeX engine: %q", string(err))
}

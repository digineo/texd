package tex

import "fmt"

type Engine struct {
	Name  string
	Flags []string
}

var engines = []Engine{
	{"xelatex", []string{"-pdflua"}},
	{"pdflatex", []string{"-pdfxe"}},
	{"lualatex", []string{"-pdf"}},
}
var DefaultEngine = engines[0]

func SupportedEngines() (e []string) {
	for _, engine := range engines {
		e = append(e, engine.Name)
	}
	return
}

func ParseTeXEngine(s string) (Engine, error) {
	for _, engine := range engines {
		if engine.Name == s {
			return engine, nil
		}
	}
	return Engine{}, ErrUnsupportedEngine(s)
}

func (x Engine) String() string {
	return x.Name
}

type ErrUnsupportedEngine string

func (err ErrUnsupportedEngine) Error() string {
	return fmt.Sprintf("unsupported TeX engine: %q", string(err))
}

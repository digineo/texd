package tex

import "fmt"

type Engine struct {
	name  string
	flags []string
}

func NewEngine(name string, latexmkFlags ...string) Engine {
	return Engine{name, latexmkFlags}
}

func (e Engine) Name() string    { return e.name }
func (e Engine) String() string  { return e.name }
func (e Engine) Flags() []string { return e.flags }

var (
	engines = []Engine{
		NewEngine("xelatex", "-pdfxe"),
		NewEngine("pdflatex", "-pdf"),
		NewEngine("lualatex", "-pdflua"),
	}
	DefaultEngine = engines[0]
)

func SupportedEngines() (e []string) {
	for _, engine := range engines {
		e = append(e, engine.name)
	}
	return
}

func SetDefaultEngine(name string) error {
	engine, err := ParseEngine(name)
	if err != nil {
		return err
	}
	DefaultEngine = engine
	return nil
}

func ParseEngine(name string) (Engine, error) {
	for _, engine := range engines {
		if engine.name == name {
			return engine, nil
		}
	}
	return Engine{}, ErrUnsupportedEngine(name)
}

type ErrUnsupportedEngine string

func (err ErrUnsupportedEngine) Error() string {
	return fmt.Sprintf("unsupported TeX engine: %q", string(err))
}

var LatexmkDefaultFlags = []string{
	"-cd",           // change to directory
	"-silent",       // reduce diagnostics and run engine with -interaction=batchmode
	"-pv-", "-pvc-", // turn off (continuous) file previewing
}

// latexmk builds a command line for latexmk invocation.
func (engine Engine) LatexmkCmd(main string) []string {
	lenDefaults := len(LatexmkDefaultFlags)
	flags := engine.Flags()
	lenFlags := len(flags)

	cmd := make([]string, 1+lenDefaults+lenFlags+1)
	cmd[0] = "latexmk"
	copy(cmd[1:], LatexmkDefaultFlags)
	copy(cmd[1+lenDefaults:], flags)
	cmd[1+lenDefaults+lenFlags] = main

	return cmd
}

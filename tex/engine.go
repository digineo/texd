package tex

import (
	"fmt"
)

type Engine struct {
	name  string
	flags []string
}

func NewEngine(name string, latexmkFlags ...string) Engine {
	return Engine{name: name, flags: latexmkFlags}
}

func (e Engine) Name() string   { return e.name }
func (e Engine) String() string { return e.name }
func (e Engine) Flags() []string {
	switch shellEscaping {
	case RestrictedShellEscape:
		return e.flags
	case AllowedShellEscape:
		return append([]string{"-shell-escape"}, e.flags...)
	case ForbiddenShellEscape:
		return append([]string{"-no-shell-escape"}, e.flags...)
	}
	panic("not reached")
}

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

// LatexmkCmd builds a command line for latexmk invocation.
func (e Engine) LatexmkCmd(main string) []string {
	lenDefaults := len(LatexmkDefaultFlags)
	flags := e.Flags()
	lenFlags := len(flags)

	cmd := make([]string, 1+lenDefaults+lenFlags+1)
	cmd[0] = "latexmk"
	copy(cmd[1:], LatexmkDefaultFlags)
	copy(cmd[1+lenDefaults:], flags)
	cmd[1+lenDefaults+lenFlags] = main

	return cmd
}

type ShellEscape int

const (
	RestrictedShellEscape ShellEscape = iota // allows restricted command execution (e.g. bibtex)
	AllowedShellEscape                       // allow arbitraty command execution
	ForbiddenShellEscape                     // prohibit execution of any commands
	maxShellEscape                           // must be last
)

type ErrUnexpectedShellEscape ShellEscape

func (err ErrUnexpectedShellEscape) Error() string {
	return fmt.Sprintf("unexpected shell escaping value: %d", int(err))
}

var shellEscaping = RestrictedShellEscape

// SetShellEscaping globally configures which external programs the TeX compiler
// is allowd to execute. By default, only a restricted set of external programs
// are allowed, such as bibtex, kpsewhich, etc.
//
// When set to [ShellEscapeAllowed], the `-shell-escape` flag is passed to
// `latexmk`. Note that this enables arbitrary command execution, and consider
// the security implications.
//
// To disable any external command execution, use [ShellEscapeForbidden]. This
// is equivalent to passing `-no-shell-escape` to `latexmk`.

// Use [RestrictedShellEscape] to reset to the default value.
//
// Calling this with an unexpected value will return an error.
func SetShellEscaping(value ShellEscape) error {
	if value < 0 || value >= maxShellEscape {
		return ErrUnexpectedShellEscape(value)
	}
	shellEscaping = value
	return nil
}

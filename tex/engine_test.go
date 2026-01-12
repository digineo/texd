package tex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_LatexmkCmd(t *testing.T) {
	t.Cleanup(func() { shellEscaping = 0 })

	const mainInput = "test.tex"

	for _, esc := range []ShellEscape{RestrictedShellEscape, AllowedShellEscape, ForbiddenShellEscape} {
		require.NoError(t, SetShellEscaping(esc))

		latexmk := []string{"latexmk", "-cd", "-silent", "-pv-", "-pvc-"}
		shell := "restricted"
		switch esc {
		case RestrictedShellEscape:
			// nothing to do
		case AllowedShellEscape:
			latexmk = append(latexmk, "-shell-escape")
			shell = "allowed"
		case ForbiddenShellEscape:
			latexmk = append(latexmk, "-no-shell-escape")
			shell = "forbidden"
		}

		for name, flags := range map[string][]string{
			"nil":    nil,
			"empty":  {},
			"single": {"-single-flag"},
			"multi":  {"-multiple", "-flags"},
		} {
			t.Run(shell+"_"+name, func(t *testing.T) {
				expected := make([]string, 0, len(latexmk)+len(flags)+1)
				expected = append(expected, latexmk...)
				expected = append(expected, flags...)
				expected = append(expected, mainInput)

				cmd := NewEngine("noname", flags...).LatexmkCmd(mainInput)
				assert.EqualValues(t, expected, cmd)
			})
		}
	}
}

func TestSetShellEscape(t *testing.T) {
	require := require.New(t)
	t.Cleanup(func() { shellEscaping = 0 })

	require.NoError(SetShellEscaping(RestrictedShellEscape))
	require.NoError(SetShellEscaping(AllowedShellEscape))
	require.NoError(SetShellEscaping(ForbiddenShellEscape))

	require.EqualError(SetShellEscaping(-1), "unexpected shell escaping value: -1")
	require.EqualError(SetShellEscaping(maxShellEscape), "unexpected shell escaping value: 3")
	require.EqualError(SetShellEscaping(maxShellEscape+1), "unexpected shell escaping value: 4")
}

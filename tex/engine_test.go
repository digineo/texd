package tex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEngine_LatexmkCmd(t *testing.T) {
	t.Parallel()

	const mainInput = "test.tex"

	for _, tc := range []struct {
		flags    []string
		expected []string
	}{
		{
			flags:    nil,
			expected: []string{"latexmk", "-cd", "-silent", "-pv-", "-pvc-", mainInput},
		}, {
			flags:    []string{},
			expected: []string{"latexmk", "-cd", "-silent", "-pv-", "-pvc-", mainInput},
		}, {
			flags:    []string{"-single"},
			expected: []string{"latexmk", "-cd", "-silent", "-pv-", "-pvc-", "-single", mainInput},
		}, {
			flags:    []string{"-multiple", "-flags"},
			expected: []string{"latexmk", "-cd", "-silent", "-pv-", "-pvc-", "-multiple", "-flags", mainInput},
		},
	} {
		cmd := NewEngine("noname", tc.flags...).LatexmkCmd(mainInput)
		assert.EqualValues(t, tc.expected, cmd)
	}
}

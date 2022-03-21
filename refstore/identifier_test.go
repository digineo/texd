package refstore

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIdentifier(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		input string // input to ParseIdentifier
		raw   string // result of Identifier.Raw()
		err   string // expected error
	}{
		"input empty": {
			input: "",
			err:   "invalid identifier: unexpected input length",
		},
		"input too short": {
			input: "sha256:aaaaaaaaa-bbbbbbbbb000000000zzzzzzzzz-ABCD",
			err:   "invalid identifier: unexpected input length",
		},
		"input too long": {
			input: "sha256:aaaaaaaaa-bbbbbbbbb000000000zzzzzzzzz-ABCDEXYX",
			err:   "invalid identifier: unexpected input length",
		},
		"missing prefix": {
			input: "md4242:aaaaaaaaa-bbbbbbbbb000000000zzzzzzzzz-ABCDE",
			err:   "invalid identifier: missing hash prefix",
		},
		"invalid encoding": {
			input: "sha256:aaaaaaaaa-bbbbbbbbb000000000zzzzzzzz!-ABCDE",
			err:   "invalid identifier: decoding failed: illegal base64 data at input byte 36",
		},
		"unexpected padding": {
			input: "sha256:aaaaaaaaa-bbbbbbbbb000000000zzzzzzzzz-ABCDEX",
			err:   "invalid identifier: unexpected non-padding character at the end",
		},
		"url encoding no padding": {
			input: "sha256:aaaaaaaaa-bbbbbbbbb000000000zzzzzzzzz-ABCDE",
			raw:   "aaaaaaaaa-bbbbbbbbb000000000zzzzzzzzz-ABCDE",
		},
		"url encoding": {
			input: "sha256:aaaaaaaaa-bbbbbbbbb000000000zzzzzzzzz-ABCDE=",
			raw:   "aaaaaaaaa-bbbbbbbbb000000000zzzzzzzzz-ABCDE",
		},
		"std encoding no padding": {
			input: "sha256:aaaaaaaaa+bbbbbbbbb000000000zzzzzzzzz+ABCDE",
			raw:   "aaaaaaaaa-bbbbbbbbb000000000zzzzzzzzz-ABCDE",
		},
		"std encoding": {
			input: "sha256:aaaaaaaaa+bbbbbbbbb000000000zzzzzzzzz+ABCDE=",
			raw:   "aaaaaaaaa-bbbbbbbbb000000000zzzzzzzzz-ABCDE",
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			id, err := ParseIdentifier([]byte(tc.input))
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, string(prefix)+tc.raw, id.String())
				assert.Equal(t, tc.raw, id.Raw())
			}
		})
	}
}

var identifierTestCases = map[string][]byte{
	"47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU": nil,
	"fHRzUZbdP9Y1OoBcDRoS3dloouSYvuMypjD7Gzk_LAA": []byte(`\documentclass{article}\begin{document}\end{document}`),
}

func TestNewIdentifier(t *testing.T) {
	t.Parallel()

	for id, contents := range identifierTestCases {
		actual := NewIdentifier(contents)
		assert.Equal(t, id, actual.Raw())
		assert.Equal(t, Identifier(id), actual)
	}
}

func TestReadIdentifier(t *testing.T) {
	t.Parallel()

	for id, contents := range identifierTestCases {
		buf := bytes.NewReader(contents)

		actual, err := ReadIdentifier(buf)
		assert.NoError(t, err)
		assert.Equal(t, id, actual.Raw())
		assert.Equal(t, Identifier(id), actual)
	}
}

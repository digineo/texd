package exec

import (
	"errors"
	"testing"

	"github.com/digineo/texd/tex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDocument struct {
	wd      string
	wdErr   error
	main    string
	mainErr error
}

var _ Document = (*mockDocument)(nil)

// methods of tex.Document needed for this test.
func (m *mockDocument) WorkingDirectory() (string, error) { return m.wd, m.wdErr }
func (m *mockDocument) MainInput() (string, error)        { return m.main, m.mainErr }
func (*mockDocument) Engine() tex.Engine                  { return tex.DefaultEngine }

// methods required to satisfy the Document interface.
func (*mockDocument) Image() string { return "" }

func TestBaseExec_extract(t *testing.T) {
	dirErr := errors.New("dir error")
	mainErr := errors.New("main error")

	for _, tc := range []struct {
		expectedDir   string
		expectedError string
		mockDocument
	}{
		{
			expectedDir:  "",
			mockDocument: mockDocument{"", nil, "", nil},
		}, {
			expectedDir:  "/",
			mockDocument: mockDocument{"/", nil, "", nil},
		}, {
			expectedDir:  "",
			mockDocument: mockDocument{"", nil, "/", nil},
		}, {
			expectedDir:  "a",
			mockDocument: mockDocument{"a", nil, "b", nil},
		}, {
			expectedDir:  "/a",
			mockDocument: mockDocument{"/a", nil, "b", nil},
		}, {
			expectedError: "dir error",
			mockDocument:  mockDocument{"", dirErr, "b", nil},
		}, {
			expectedError: "main error",
			mockDocument:  mockDocument{"", dirErr, "b", mainErr},
		},
	} {
		subject := baseExec{&tc.mockDocument}
		dir, cmd, err := subject.extract()
		if tc.expectedError == "" {
			require.NoError(t, err)
			assert.Equal(t, tc.expectedDir, dir)
			assert.Equal(t, "latexmk", cmd[0])
			assert.Equal(t, tc.mockDocument.main, cmd[len(cmd)-1])
		} else {
			require.EqualError(t, err, tc.expectedError)
		}
	}
}

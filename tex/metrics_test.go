package tex

import (
	"bytes"
	"sort"
	"testing"

	"github.com/digineo/texd/xlog"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileCategory_FromName(t *testing.T) {
	for _, tc := range []struct {
		expected fileCategory
		names    []string
	}{
		// obvious good cases
		{texFile, []string{"input.tex", "class.cls", "package.sty", "koma.lco", "koma.lco", "lit.bib", "precompiled.bbl"}},
		{assetFile, []string{"img.png", "img.jpg", "img.jpeg", "img.gif"}},
		{assetFile, []string{"logo.pdf", "logo.eps", "logo.svg"}},
		{assetFile, []string{"font.otf", "font.ttf", "font.mf", "font.pfm", "font.pfb"}},
		{dataFile, []string{"data.csv", "data.xml", "data.json"}},
		{otherFile, []string{"web.html", "data.dat"}},

		{texFile, []string{"a.tex", "a.b.tex", "a/b.tex", "/a/b/c.tex"}},

		// edge cases
		{otherFile, []string{"", ".", ".tex", "tex.", "/a/.tex"}},
	} {
		for _, name := range tc.names {
			actual := categoryFromName(name)
			assert.Equal(t, tc.expected, actual,
				"expected name %q to be of cat %q, got %q", name, tc.expected, actual)
		}
	}
}

func TestFileCategory_String(t *testing.T) {
	for cat, s := range map[fileCategory]string{
		texFile:   "tex",
		dataFile:  "data",
		assetFile: "asset",
		otherFile: "other",
		0x42:      "%!unknown(0x42)",
	} {
		assert.Equal(t, s, cat.String())
	}
}

func TestMetrics(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	doc := NewDocument(xlog.NewNop(), DefaultEngine, "")
	doc.(*document).fs = afero.NewMemMapFs()
	for name, size := range map[string]int{
		"input.tex":              10,
		"common/pkg.sty":         20,
		"common/logo.pdf":        30,
		"common/letter.cls":      40,
		"common/Hausschrift.otf": 50,
		"recipients.csv":         60,
		"recipients.xlsx":        70,
	} {
		w, err := doc.NewWriter(name)
		require.NoError(err, "can't create writer for %s", name)
		n, err := w.Write(bytes.Repeat([]byte("a"), size))
		require.NoError(err, "writing to %s failed", name)
		require.Equal(size, n, "written only %d of %d bytes for %s", n, size, name)
		require.NoError(w.Close(), "closing writer for %s failed", name)
	}
	require.NoError(doc.SetMainInput("input.tex"))

	m := doc.Metrics()
	assert.Equal(-1, m.Result)

	assertSortedEqual := func(cat string, expected, actual []int) {
		t.Helper()
		sort.Ints(actual)
		assert.EqualValues(expected, actual, cat)
	}

	assertSortedEqual("TexFiles", []int{10, 20, 40}, m.TexFiles)
	assertSortedEqual("AssetFiles", []int{30, 50}, m.AssetFiles)
	assertSortedEqual("DataFiles", []int{60}, m.DataFiles)
	assertSortedEqual("OtherFiles", []int{70}, m.OtherFiles)

	w, err := doc.NewWriter("input.pdf")
	require.NoError(err)
	n, err := w.Write([]byte("123"))
	require.NoError(err)
	assert.Equal(3, n)
	require.NoError(w.Close())

	m = doc.Metrics()
	assert.Equal(3, m.Result)
}

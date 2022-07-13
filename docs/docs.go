package docs

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Depado/bfchroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	bf "github.com/russross/blackfriday/v2"
	"gopkg.in/yaml.v3"
)

//go:embed docs.yml *.md **/*.md
var sources embed.FS

type page struct {
	Title       string
	Breadcrumbs []string
	Body        string
	File        string
	Route       string
	Children    []*page
}

var root = func() page {
	structure, err := sources.Open("docs.yml")
	if err != nil {
		panic(err)
	}
	defer structure.Close()

	var menu page
	dec := yaml.NewDecoder(structure)
	dec.KnownFields(true)
	if err := dec.Decode(&menu); err != nil {
		panic(err)
	}

	menu.init()
	return menu
}()

func (pg *page) init(crumbs ...string) {
	if pg.File != "" {
		if r := strings.TrimSuffix(pg.File, ".md"); r == "index" {
			pg.Route = ""
		} else {
			pg.Route = "/" + r
		}

		pg.parseFile()
	}
	if pg.Title != "" {
		pg.Breadcrumbs = append(pg.Breadcrumbs, pg.Title)
	}
	for _, child := range pg.Children {
		child.init(pg.Breadcrumbs...)
	}
}

func (pg *page) parseFile() {
	body, err := sources.ReadFile(pg.File)
	if err != nil {
		panic(err)
	}

	r := bfchroma.NewRenderer(
		bfchroma.WithoutAutodetect(),
		bfchroma.ChromaOptions(
			html.WithLineNumbers(true),
		),
		bfchroma.Extend(bf.NewHTMLRenderer(bf.HTMLRendererParameters{
			Flags: bf.CommonHTMLFlags & ^bf.UseXHTML & ^bf.CompletePage,
		})),
	)
	parser := bf.New(
		bf.WithExtensions(bf.CommonExtensions),
		bf.WithRenderer(r),
	)

	ast := parser.Parse(body)
	var buf bytes.Buffer
	var inH1 bool

	ast.Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
		switch node.Type {
		case bf.Heading:
			inH1 = entering && node.HeadingData.Level == 1 && pg.Title == ""
		case bf.Text:
			if inH1 {
				pg.Title = string(node.Literal)
			}
		case bf.Link:
			if entering && bytes.HasPrefix(node.LinkData.Destination, []byte("./")) {
				node.LinkData.Destination = bytes.TrimSuffix(node.LinkData.Destination, []byte(".md"))
			}
		}
		return r.RenderNode(&buf, node, entering)
	})

	pg.Body = buf.String()
}

func (pg *page) Dump(w io.Writer) {
	fmt.Fprintf(w, "- %s (%s)\n", pg.Title, pg.Route)
	fmt.Fprintln(w, pg.Body)
	fmt.Fprintln(w)

	for _, c := range pg.Children {
		c.Dump(w)
	}
}

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "%#v\n\n", r.URL)

		root.Dump(w)
	})
}

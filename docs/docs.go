package docs

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"

	toc "github.com/abhinav/goldmark-toc"
	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/digineo/texd"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"gopkg.in/yaml.v3"
)

//go:embed *.md **/*.md
var sources embed.FS

//go:embed docs.yml
var config []byte

//go:embed docs.html
var rawLayout string
var tplLayout = template.Must(template.New("layout").Parse(rawLayout))

type page struct {
	Title       string
	Breadcrumbs []string
	TOC         *toc.TOC
	CSS         []byte
	Body        []byte
	File        string
	Route       string
	Children    []*page
}

type pageRoutes map[string]*page

func getRoutes(urlPrefix string) (pageRoutes, error) {
	var menu page
	dec := yaml.NewDecoder(bytes.NewReader(config))
	dec.KnownFields(true)
	if err := dec.Decode(&menu); err != nil {
		return nil, err
	}

	urlPrefix = strings.TrimSuffix(urlPrefix, "/")
	return menu.init(urlPrefix, make(pageRoutes))
}

func (pg *page) init(urlPrefix string, r pageRoutes, crumbs ...string) (pageRoutes, error) {
	if pg.File != "" {
		if r := strings.TrimSuffix(pg.File, ".md"); r == "README" {
			pg.Route = urlPrefix
		} else {
			pg.Route = urlPrefix + "/" + r
		}
		r[pg.Route] = pg
		err := pg.parseFile(urlPrefix)
		if err != nil {
			return nil, err
		}
	}
	if pg.Title != "" {
		pg.Breadcrumbs = append([]string{pg.Title}, crumbs...)
	}
	for _, child := range pg.Children {
		_, err := child.init(urlPrefix, r, pg.Breadcrumbs...)
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}

type localLinkTransformer struct {
	prefix string
}

var _ parser.ASTTransformer = (*localLinkTransformer)(nil)

func (link *localLinkTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindLink {
			if l, ok := n.(*ast.Link); ok {
				link.rewrite(l)
			}
		}
		return ast.WalkContinue, nil
	})
}

const (
	localLinkPrefix = "./"
	localLinkSuffix = ".md"
)

func (link *localLinkTransformer) rewrite(l *ast.Link) {
	dst := string(l.Destination)
	if strings.HasPrefix(dst, localLinkPrefix) && strings.HasSuffix(dst, localLinkSuffix) {
		dst = strings.TrimPrefix(dst, localLinkPrefix)
		dst = strings.TrimSuffix(dst, localLinkSuffix)
		l.Destination = []byte(link.prefix + "/" + dst)
	}
}

var sanitize = func() func(io.Reader) *bytes.Buffer {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("class").Globally()
	return p.SanitizeReader
}()

func (pg *page) parseFile(urlPrefix string) error {
	raw, err := sources.ReadFile(pg.File)
	if err != nil {
		return err
	}

	var css, body bytes.Buffer
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.PrioritizedValue{
				Value:    &localLinkTransformer{urlPrefix},
				Priority: 999,
			}),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithCSSWriter(&css),
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
					chromahtml.WithClasses(true),
				),
			),
		),
	)

	doc := md.Parser().Parse(text.NewReader(raw))
	tree, err := toc.Inspect(doc, raw)
	if err != nil {
		return err
	}
	if pg.Title == "" {
		if len(tree.Items) > 0 {
			pg.Title = string(tree.Items[0].Title)
		}
	}
	if err := md.Renderer().Render(&body, raw, doc); err != nil {
		return err
	}
	pg.TOC = tree
	pg.CSS = css.Bytes()
	pg.Body = sanitize(&body).Bytes()
	return nil
}

func Handler(prefix string) (http.Handler, error) {
	type pageVars struct {
		Version string
		Title   string
		CSS     template.CSS
		Content template.HTML
	}

	routes, err := getRoutes(prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to build docs: %w", err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pg := routes[r.URL.Path]
		if pg == nil {
			http.NotFound(w, r)
			return
		}

		var buf bytes.Buffer
		err := tplLayout.Execute(&buf, &pageVars{
			Version: texd.Version(),
			Title:   strings.Join(pg.Breadcrumbs, " · "),
			CSS:     template.CSS(pg.CSS),
			Content: template.HTML(pg.Body),
		})

		if err != nil {
			log.Println(err)
			code := http.StatusInternalServerError
			http.Error(w, http.StatusText(code), code)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = buf.WriteTo(w)
	}), nil
}

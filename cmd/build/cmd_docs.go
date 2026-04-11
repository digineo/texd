package main

import (
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extAst "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

//go:embed template.html
var templateHTML string

type pageData struct {
	Title       string
	Content     template.HTML
	CurrentPage string
	NavSections []navSection
}

// frontmatter represents YAML frontmatter in markdown files
type frontmatter struct {
	Title    string `yaml:"title"`    // Page title
	NavTitle string `yaml:"navTitle"` // Title shown in navigation (optional, defaults to Title)
	Section  string `yaml:"section"`  // Section name (e.g., "Configuration", "API Reference")
	Order    int    `yaml:"order"`    // Order within section
}

// navSection represents a navigation section with items
type navSection struct {
	Title string
	Items []navItem
}

// navItem represents a single navigation link
type navItem struct {
	Title string
	Href  string
	Slug  string
}

var linkRewriter = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\.md(#[^)]*)?\)`)

// parseFrontmatter extracts YAML frontmatter from markdown content
// Frontmatter must be at the beginning of the file, surrounded by "---" lines
func parseFrontmatter(content string) (*frontmatter, string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		// No frontmatter found
		return nil, content, nil
	}

	// Find closing "---"
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return nil, content, fmt.Errorf("unclosed frontmatter")
	}

	// Parse YAML
	fm := &frontmatter{}
	yamlContent := strings.Join(lines[1:endIdx], "\n")
	if err := yaml.Unmarshal([]byte(yamlContent), fm); err != nil {
		return nil, content, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Return frontmatter and remaining content
	remainingContent := strings.Join(lines[endIdx+1:], "\n")
	return fm, remainingContent, nil
}

// cssClassTransformer adds Bootstrap CSS classes to specific HTML elements
type cssClassTransformer struct{}

func (t *cssClassTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n.Kind() {
		case extAst.KindTable:
			// Add Bootstrap table classes to tables
			n.SetAttributeString("class", []byte("table table-striped border"))
		}

		return ast.WalkContinue, nil
	})
}

// customCodeBlockRenderer renders code blocks with Bootstrap classes
type customCodeBlockRenderer struct{ html.Config }

func (r *customCodeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderCodeBlock)
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
}

func (r *customCodeBlockRenderer) renderCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<pre class="bg-light border rounded p-3 overflow-x-auto mw-100">`)
		if n, ok := node.(*ast.FencedCodeBlock); ok {
			language := n.Language(source)
			if len(language) > 0 {
				_, _ = w.WriteString(`<code class="language-`)
				_, _ = w.Write(language)
				_, _ = w.WriteString(`">`)
			} else {
				_, _ = w.WriteString("<code>")
			}
		} else {
			_, _ = w.WriteString("<code>")
		}

		// Write the code content
		l := node.Lines()
		for i := range l.Len() {
			line := l.At(i)
			_, _ = w.Write(util.EscapeHTML(line.Value(source)))
		}
	} else {
		_, _ = w.WriteString("</code></pre>\n")
	}
	return ast.WalkContinue, nil
}

type customBlockquoteRenderer struct{ html.Config }

func (r *customBlockquoteRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindBlockquote, r.renderBlockquote)
}

func (r *customBlockquoteRenderer) renderBlockquote(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<blockquote class="border-start border-3 ms-3 ps-3 fst-italic">`)
	} else {
		_, _ = w.WriteString("</blockquote>")
	}
	return ast.WalkContinue, nil
}

// docMeta holds document metadata during processing
type docMeta struct {
	filename string
	fm       *frontmatter
	content  string
}

func generateDocs(input, output string) error {
	// Create goldmark converter
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(&cssClassTransformer{}, 100),
			),
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(&customCodeBlockRenderer{}, 100),
				util.Prioritized(&customBlockquoteRenderer{}, 100),
			),
		),
	)

	// Parse template
	tmpl, err := template.New("doc").Parse(templateHTML)
	if err != nil {
		return fmt.Errorf("gendocs: failed to parse template: %w", err)
	}

	// Find all markdown files in docs/
	entries, err := os.ReadDir(input)
	if err != nil {
		return fmt.Errorf("gendocs: failed to read docs directory: %w", err)
	}

	// Ensure output path exists
	if _, err := os.Stat(output); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(output, 0o755); err != nil {
			return fmt.Errorf("gendocs: cannot create output directory %q: %w", output, err)
		}
	} else if err != nil {
		return fmt.Errorf("gendocs: cannot stat output directory %q: %w", output, err)
	}

	// First pass: collect all frontmatter to build navigation
	var docs []docMeta

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		inputPath := filepath.Join(input, entry.Name())
		content, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("gendocs: failed to read %s: %w", entry.Name(), err)
		}

		fm, remaining, err := parseFrontmatter(string(content))
		if err != nil {
			return fmt.Errorf("gendocs: error parsing frontmatter in %s: %w", entry.Name(), err)
		}

		// If no frontmatter, skip this file from navigation
		if fm == nil {
			logf("warning: %s has no frontmatter, skipping from navigation", entry.Name())
			continue
		}

		docs = append(docs, docMeta{
			filename: entry.Name(),
			fm:       fm,
			content:  remaining,
		})
	}

	// Build navigation structure
	navSections := buildNavigation(docs)

	// Second pass: generate HTML files
	generatedCount := 0
	for _, doc := range docs {
		inputPath := filepath.Join(input, doc.filename)
		outputPath := filepath.Join(output, strings.TrimSuffix(doc.filename, ".md")+".html")
		logf("converting %s -> %s", inputPath, outputPath)

		if err := convertMarkdownToHTML(md, tmpl, inputPath, outputPath, doc.fm, doc.content, navSections); err != nil {
			return fmt.Errorf("gendocs: failed to convert %s: %w", doc.filename, err)
		}
		generatedCount++
	}

	if generatedCount == 0 {
		return fmt.Errorf("gendocs: no markdown files with frontmatter found in %s", input)
	}
	return nil
}

// buildNavigation creates navigation sections from document metadata
func buildNavigation(docs []docMeta) []navSection {
	// Group documents by section
	sections := make(map[string][]docMeta)
	for _, doc := range docs {
		section := doc.fm.Section
		sections[section] = append(sections[section], doc)
	}

	// Sort documents within each section by order
	for _, docs := range sections {
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].fm.Order < docs[j].fm.Order
		})
	}

	// Build navigation structure
	// We'll use a fixed order for sections based on common patterns
	// Empty string ("") represents pages without a section header
	sectionOrder := []string{"", "Configuration", "API Reference", "Features", "More"}
	var navSections []navSection

	for _, sectionName := range sectionOrder {
		docs, ok := sections[sectionName]
		if !ok {
			continue
		}

		var items []navItem
		for _, doc := range docs {
			slug := strings.TrimSuffix(doc.filename, ".md")
			navTitle := doc.fm.NavTitle
			if navTitle == "" {
				navTitle = doc.fm.Title
			}

			items = append(items, navItem{
				Title: navTitle,
				Href:  "/docs/" + slug + ".html",
				Slug:  slug,
			})
		}

		navSections = append(navSections, navSection{
			Title: sectionName,
			Items: items,
		})
	}

	// Add any remaining sections not in the predefined order
	for sectionName, docs := range sections {
		if slices.Contains(sectionOrder, sectionName) {
			continue
		}

		var items []navItem
		for _, doc := range docs {
			slug := strings.TrimSuffix(doc.filename, ".md")
			navTitle := doc.fm.NavTitle
			if navTitle == "" {
				navTitle = doc.fm.Title
			}

			items = append(items, navItem{
				Title: navTitle,
				Href:  "/docs/" + slug + ".html",
				Slug:  slug,
			})
		}

		navSections = append(navSections, navSection{
			Title: sectionName,
			Items: items,
		})
	}

	return navSections
}

func convertMarkdownToHTML(md goldmark.Markdown, tmpl *template.Template, inputPath, outputPath string, fm *frontmatter, mdContent string, navSections []navSection) error {
	// Rewrite .md links to .html
	contentStr := linkRewriter.ReplaceAllString(mdContent, "[$1]($2.html$3)")

	// Convert markdown to HTML
	var buf strings.Builder
	if err := md.Convert([]byte(contentStr), &buf); err != nil {
		return fmt.Errorf("failed to convert markdown: %w", err)
	}

	// Use title from frontmatter if available, otherwise extract from H1
	title := fm.Title
	if title == "" {
		title = extractTitle(contentStr)
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(inputPath), ".md")
		title = strings.ReplaceAll(title, "-", " ")
		title = cases.Title(language.English).String(title)
	}

	// Determine current page name (for sidebar highlighting)
	currentPage := strings.TrimSuffix(filepath.Base(inputPath), ".md")

	// Prepare template data
	data := pageData{
		Title:       title,
		Content:     template.HTML(buf.String()),
		CurrentPage: currentPage,
		NavSections: navSections,
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	// Execute template
	if err := tmpl.Execute(outFile, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func extractTitle(content string) string {
	// Look for first H1 heading (# Title)
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

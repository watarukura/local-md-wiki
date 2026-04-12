package main

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/pflag"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v2"
)

//go:embed public/*
var publicFS embed.FS

var (
	version = "1.0.0"
	commit  = "none"
	date    = "unknown"
)

type PageInfo struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

type PageResponse struct {
	Name        string      `json:"name"`
	Markdown    string      `json:"markdown"`
	Frontmatter interface{} `json:"frontmatter"`
	HTML        string      `json:"html"`
	Backlinks   []string    `json:"backlinks"`
	TwoHop      []TwoHop    `json:"twoHop"`
}

type TwoHop struct {
	Page  string `json:"page"`
	Score int    `json:"score"`
}

var (
	pagesDir   = "pages"
	uploadsDir = filepath.Join("public", "uploads")
	md         goldmark.Markdown
)

func init() {
	md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github-dark"),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	var (
		showVersion bool
		port        string
	)

	pflag.BoolVarP(&showVersion, "version", "v", false, "show version")
	pflag.StringVarP(&port, "port", "p", getEnv("LOCAL_MD_WIKI_PORT", "3000"), "port to listen on")
	pflag.StringVarP(&pagesDir, "pages", "d", getEnv("LOCAL_MD_WIKI_PAGES_DIR", "pages"), "pages directory")
	pflag.StringVarP(&uploadsDir, "uploads", "u", getEnv("LOCAL_MD_WIKI_UPLOADS_DIR", filepath.Join("public", "uploads")), "uploads directory")
	pflag.Parse()

	if showVersion {
		fmt.Printf("mdwiki version %s (%s) built at %s\n", version, commit, date)
		return
	}

	// Ensure directories exist
	_ = os.MkdirAll(pagesDir, 0755)
	_ = os.MkdirAll(uploadsDir, 0755)

	e := echo.New()

	e.Use(middleware.Logger()) // nolint:staticcheck
	e.Use(middleware.Recover())

	// Static files from embedded FS
	subFS, _ := fs.Sub(publicFS, "public")
	e.Static("/static/uploads", uploadsDir)
	e.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(subFS)))))
	e.GET("/favicon.ico", echo.WrapHandler(http.FileServer(http.FS(subFS))))

	// API
	e.GET("/api/pages", listPages)
	e.GET("/api/page", getPage)
	e.POST("/api/page", createPage)
	e.PUT("/api/page", updatePage)
	e.POST("/api/upload", uploadFile)

	// Serve index.html for all other routes
	e.GET("/*", func(c echo.Context) error {
		if strings.HasPrefix(c.Request().URL.Path, "/api/") {
			return echo.ErrNotFound
		}
		data, err := fs.ReadFile(subFS, "index.html")
		if err != nil {
			return c.String(http.StatusInternalServerError, "index.html not found")
		}
		return c.HTML(http.StatusOK, string(data))
	})

	e.Logger.Fatal(e.Start(":" + port))
}

func listPages(c echo.Context) error {
	files, err := listMarkdownFiles(pagesDir, "")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	pages := []PageInfo{}
	for _, file := range files {
		fullPath := filepath.Join(pagesDir, file)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		var data map[string]interface{}
		rest, err := frontmatter.Parse(bytes.NewReader(content), &data)
		if err != nil {
			rest = content
		}

		title := ""
		if t, ok := data["title"].(string); ok && t != "" {
			title = t
		} else {
			re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
			match := re.FindStringSubmatch(string(rest))
			if len(match) > 1 {
				title = strings.TrimSpace(match[1])
			}
		}

		if title == "" {
			title = file
		}

		pages = append(pages, PageInfo{Name: file, Title: title})
	}

	return c.JSON(http.StatusOK, map[string][]PageInfo{"pages": pages})
}

func getPage(c echo.Context) error {
	name := c.QueryParam("name")
	if name == "" {
		name = "Home.md"
	}
	name, err := normalizePageName(name)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	fullPath, err := pagePath(name)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "page not found"})
	}

	raw, err := os.ReadFile(fullPath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	var data map[string]interface{}
	content, err := frontmatter.Parse(bytes.NewReader(raw), &data)
	if err != nil {
		content = raw
	}
	if data == nil {
		data = make(map[string]interface{})
	}

	var htmlContent bytes.Buffer
	if err := md.Convert(content, &htmlContent); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	graph, _ := buildGraph()

	return c.JSON(http.StatusOK, PageResponse{
		Name:        name,
		Markdown:    string(raw),
		Frontmatter: data,
		HTML:        htmlContent.String(),
		Backlinks:   backlinksOf(name, graph),
		TwoHop:      twoHopOf(name, graph),
	})
}

func createPage(c echo.Context) error {
	var body struct {
		Name     string      `json:"name"`
		Markdown string      `json:"markdown"`
		Title    string      `json:"title"`
		Tags     interface{} `json:"tags"`
	}
	if err := c.Bind(&body); err != nil {
		return err
	}

	name, err := normalizePageName(body.Name)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	fullPath, err := pagePath(name)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if _, err := os.Stat(fullPath); err == nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "page already exists"})
	}

	var data map[string]interface{}
	content, err := frontmatter.Parse(strings.NewReader(body.Markdown), &data)
	if err != nil {
		content = []byte(body.Markdown)
	}
	if data == nil {
		data = make(map[string]interface{})
	}

	now := time.Now().Format(time.RFC3339)
	if data["title"] == nil && body.Title != "" {
		data["title"] = body.Title
	}
	if data["tags"] == nil && body.Tags != nil {
		data["tags"] = body.Tags
	}
	if data["created_at"] == nil {
		data["created_at"] = now
	}
	data["updated_at"] = now

	finalMarkdown, err := stringifyFrontmatter(content, data)
	if err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err := os.WriteFile(fullPath, []byte(finalMarkdown), 0644); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"ok": true, "name": name})
}

func updatePage(c echo.Context) error {
	var body struct {
		Name     string `json:"name"`
		Markdown string `json:"markdown"`
	}
	if err := c.Bind(&body); err != nil {
		return err
	}

	name, err := normalizePageName(body.Name)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	fullPath, err := pagePath(name)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var existingData map[string]interface{}
	if _, err := os.Stat(fullPath); err == nil {
		raw, _ := os.ReadFile(fullPath)
		_, _ = frontmatter.Parse(bytes.NewReader(raw), &existingData)
	}

	var data map[string]interface{}
	content, err := frontmatter.Parse(strings.NewReader(body.Markdown), &data)
	if err != nil {
		content = []byte(body.Markdown)
	}
	if data == nil {
		data = make(map[string]interface{})
	}

	now := time.Now().Format(time.RFC3339)
	for k, v := range existingData {
		if data[k] == nil {
			data[k] = v
		}
	}
	if data["created_at"] == nil {
		data["created_at"] = now
	}
	data["updated_at"] = now

	finalMarkdown, err := stringifyFrontmatter(content, data)
	if err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err := os.WriteFile(fullPath, []byte(finalMarkdown), 0644); err != nil {
		return err
	}

	var htmlContent bytes.Buffer
	_ = md.Convert(content, &htmlContent)
	graph, _ := buildGraph()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":        true,
		"name":      name,
		"html":      htmlContent.String(),
		"backlinks": backlinksOf(name, graph),
		"twoHop":    twoHopOf(name, graph),
	})
}

func uploadFile(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return err
	}

	src, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	ext := filepath.Ext(file.Filename)
	fileName := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), "rand", ext)
	fullPath := filepath.Join(uploadsDir, fileName)

	dst, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()

	if _, err = io.Copy(dst, src); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":  true,
		"url": fmt.Sprintf("/static/uploads/%s", fileName),
	})
}

// Helpers

func listMarkdownFiles(dir, base string) ([]string, error) {
	files := []string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		rel := filepath.Join(base, entry.Name())
		full := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			subFiles, err := listMarkdownFiles(full, rel)
			if err != nil {
				return nil, err
			}
			files = append(files, subFiles...)
		} else if strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, rel)
		}
	}
	sort.Strings(files)
	return files, nil
}

func normalizePageName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = filepath.ToSlash(name)
	name = filepath.Clean(name)

	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "../") || filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid page name")
	}

	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	return name, nil
}

func pagePath(name string) (string, error) {
	absPagesDir, _ := filepath.Abs(pagesDir)
	full := filepath.Join(absPagesDir, name)
	resolved, _ := filepath.Abs(full)

	if !strings.HasPrefix(resolved, absPagesDir) {
		return "", fmt.Errorf("invalid page path")
	}
	return resolved, nil
}

func extractInternalLinks(markdown, currentPage string) []string {
	re := regexp.MustCompile(`\[([^\]]*?)\]\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(markdown, -1)
	links := []string{}
	seen := make(map[string]bool)

	for _, match := range matches {
		href := strings.TrimSpace(match[2])
		if href == "" || regexp.MustCompile(`^[a-zA-Z][a-zA-Z\d+\-.]*:`).MatchString(href) || strings.HasPrefix(href, "#") {
			continue
		}

		href = strings.Split(href, "#")[0]
		href = strings.Split(href, "?")[0]
		if href == "" {
			continue
		}

		currentDir := filepath.Dir(currentPage)
		resolved := filepath.ToSlash(filepath.Clean(filepath.Join(currentDir, href)))

		if strings.HasPrefix(resolved, "../") || resolved == ".." || !strings.HasSuffix(resolved, ".md") {
			continue
		}

		if !seen[resolved] {
			links = append(links, resolved)
			seen[resolved] = true
		}
	}
	sort.Strings(links)
	return links
}

func buildGraph() (map[string][]string, error) {
	files, err := listMarkdownFiles(pagesDir, "")
	if err != nil {
		return nil, err
	}

	graph := make(map[string][]string)
	for _, file := range files {
		content, _ := os.ReadFile(filepath.Join(pagesDir, file))
		var data map[string]interface{}
		rest, _ := frontmatter.Parse(bytes.NewReader(content), &data)
		graph[file] = extractInternalLinks(string(rest), file)
	}
	return graph, nil
}

func backlinksOf(target string, graph map[string][]string) []string {
	backlinks := []string{}
	for page, links := range graph {
		if page == target {
			continue
		}
		for _, link := range links {
			if link == target {
				backlinks = append(backlinks, page)
				break
			}
		}
	}
	sort.Strings(backlinks)
	return backlinks
}

func twoHopOf(target string, graph map[string][]string) []TwoHop {
	outgoing := make(map[string]bool)
	for _, link := range graph[target] {
		outgoing[link] = true
	}

	scores := make(map[string]int)
	for page, links := range graph {
		if page == target {
			continue
		}
		shared := 0
		for _, link := range links {
			if outgoing[link] {
				shared++
			}
		}
		if shared > 0 {
			scores[page] = shared
		}
	}

	result := []TwoHop{}
	for page, score := range scores {
		result = append(result, TwoHop{Page: page, Score: score})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return result[i].Page < result[j].Page
	})

	return result
}

func stringifyFrontmatter(content []byte, data map[string]interface{}) (string, error) {
	if len(data) == 0 {
		return string(content), nil
	}
	fm, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return "---\n" + string(fm) + "---\n" + string(content), nil
}

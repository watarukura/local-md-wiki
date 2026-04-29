package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/pflag"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
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

type SearchResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
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

type CommitInfo struct {
	Hash    string    `json:"hash"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	Subject string    `json:"subject"`
}

type DiffInfo struct {
	Diff string `json:"diff"`
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

	e := setupServer()
	e.Logger.Fatal(e.Start(":" + port))
}

func setupServer() *echo.Echo {
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
	e.GET("/api/search", searchPages)
	e.POST("/api/page", createPage)
	e.PUT("/api/page", updatePage)
	e.POST("/api/upload", uploadFile)
	e.GET("/api/git/log", getGitLog)
	e.GET("/api/git/diff", getGitDiff)
	e.POST("/api/git/checkout", checkoutCommit)

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

	return e
}

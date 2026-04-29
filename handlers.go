package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/labstack/echo/v4"
)

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

	go gitCommit(pagesDir, fmt.Sprintf("Create page: %s", name))

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

	go gitCommit(pagesDir, fmt.Sprintf("Update page: %s", name))

	var htmlContent bytes.Buffer
	if err := md.Convert(content, &htmlContent); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

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

	baseName := filepath.Base(file.Filename)
	ext := filepath.Ext(baseName)

	allowedExts := map[string]bool{
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".svg":  true,
		".pdf":  true,
		".zip":  true,
		".md":   true,
		".txt":  true,
	}
	if !allowedExts[strings.ToLower(ext)] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file type not allowed"})
	}

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

	go gitCommit(uploadsDir, fmt.Sprintf("Upload file: %s", fileName))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":  true,
		"url": fmt.Sprintf("/static/uploads/%s", fileName),
	})
}

func searchPages(c echo.Context) error {
	query := c.QueryParam("q")
	if query == "" {
		return c.JSON(http.StatusOK, []SearchResult{})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "grep", "-rni", "--exclude-dir=.git", "--include=*.md", "--", query, pagesDir)
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()

	results := []SearchResult{}
	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		file := parts[0]
		relFile, err := filepath.Rel(pagesDir, file)
		if err == nil {
			file = relFile
		}

		lineNum := 0
		_, _ = fmt.Sscanf(parts[1], "%d", &lineNum)
		content := strings.TrimSpace(parts[2])

		results = append(results, SearchResult{
			File:    file,
			Line:    lineNum,
			Content: content,
		})
	}

	return c.JSON(http.StatusOK, results)
}

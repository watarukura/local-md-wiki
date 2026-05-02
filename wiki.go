package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/adrg/frontmatter"
	"gopkg.in/yaml.v2"
)

func listMarkdownFiles(dir, base string) ([]string, error) {
	files := []string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == ".git" {
			continue
		}
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
		links := extractInternalLinks(string(rest), file)
		seen := make(map[string]bool)
		for _, link := range links {
			seen[link] = true
		}

		if tags, ok := data["tags"].([]interface{}); ok {
			currentDir := filepath.Dir(file)
			for _, tag := range tags {
				if t, ok := tag.(string); ok {
					// Treat tags as relative links to the current directory,
					// matching the behavior of the frontend.
					resolved := filepath.ToSlash(filepath.Clean(filepath.Join(currentDir, t+".md")))
					if !strings.HasPrefix(resolved, "../") && resolved != ".." {
						if !seen[resolved] {
							links = append(links, resolved)
							seen[resolved] = true
						}
					}
				}
			}
		}

		graph[file] = links
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

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnv(t *testing.T) {
	err := os.Setenv("TEST_KEY", "test_value")
	assert.NoError(t, err)
	defer func() { _ = os.Unsetenv("TEST_KEY") }()

	assert.Equal(t, "test_value", getEnv("TEST_KEY", "fallback"))
	assert.Equal(t, "fallback", getEnv("NON_EXISTENT_KEY", "fallback"))
}

func TestNormalizePageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		err      bool
	}{
		{"Home", "Home.md", false},
		{"Home.md", "Home.md", false},
		{"  Sub Page  ", "Sub Page.md", false},
		{"", "", true},
		{"..", "", true},
		{"../forbidden", "", true},
		{"/absolute", "", true},
		{"sub/page", "sub/page.md", false},
	}

	for _, tt := range tests {
		res, err := normalizePageName(tt.input)
		if tt.err {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, res)
		}
	}
}

func TestPagePath(t *testing.T) {
	// pagesDir is "pages" by default
	originalPagesDir := pagesDir
	defer func() { pagesDir = originalPagesDir }()
	pagesDir = "tests/pages"
	err := os.MkdirAll(pagesDir, 0755)
	assert.NoError(t, err)

	tests := []struct {
		input string
		err   bool
	}{
		{"Home.md", false},
		{"sub/page.md", false},
		{"../outside.md", true},
	}

	for _, tt := range tests {
		res, err := pagePath(tt.input)
		if tt.err {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Contains(t, res, tt.input)
		}
	}
}

func TestExtractInternalLinks(t *testing.T) {
	markdown := `
[Internal](Internal.md)
[Another](Another.md)
[External](https://google.com)
[Anchor](#anchor)
[Invalid](invalid)
[Deep](../Deep.md)
[Same Page](.)
[Empty]()
[With Query](Query.md?q=1)
[With Hash](Hash.md#header)
`
	links := extractInternalLinks(markdown, "Home.md")
	assert.ElementsMatch(t, []string{"Another.md", "Internal.md", "Query.md", "Hash.md"}, links)
}

func TestBacklinksOf(t *testing.T) {
	graph := map[string][]string{
		"A.md": {"B.md", "C.md"},
		"B.md": {"C.md"},
		"C.md": {"A.md"},
	}

	assert.ElementsMatch(t, []string{"A.md", "B.md"}, backlinksOf("C.md", graph))
	assert.ElementsMatch(t, []string{"C.md"}, backlinksOf("A.md", graph))
	assert.ElementsMatch(t, []string{}, backlinksOf("D.md", graph))
}

func TestTwoHopOf(t *testing.T) {
	graph := map[string][]string{
		"A.md": {"B.md"},
		"C.md": {"B.md"},
		"D.md": {"B.md", "E.md"},
	}

	// Two-hop for A.md:
	// A links to B.
	// C also links to B. So C is a 2-hop (shared link B).
	// D also links to B. So D is a 2-hop (shared link B).
	res := twoHopOf("A.md", graph)
	pages := []string{}
	for _, r := range res {
		pages = append(pages, r.Page)
	}
	assert.Contains(t, pages, "C.md")
	assert.Contains(t, pages, "D.md")
}

func TestStringifyFrontmatter(t *testing.T) {
	content := []byte("body content")
	data := map[string]interface{}{"title": "Test"}

	res, err := stringifyFrontmatter(content, data)
	assert.NoError(t, err)
	assert.Contains(t, res, "---")
	assert.Contains(t, res, "title: Test")
	assert.Contains(t, res, "body content")

	res, err = stringifyFrontmatter(content, nil)
	assert.NoError(t, err)
	assert.Equal(t, "body content", res)

	res, err = stringifyFrontmatter(content, map[string]interface{}{})
	assert.NoError(t, err)
	assert.Equal(t, "body content", res)
}

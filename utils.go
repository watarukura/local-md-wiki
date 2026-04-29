package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
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

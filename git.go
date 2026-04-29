package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

func gitCommit(dir, message string) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Printf("failed to get absolute path for %s: %v\n", dir, err)
		return
	}

	// 10 seconds timeout for git operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize git repository if not exists
	if _, err := os.Stat(filepath.Join(absDir, ".git")); os.IsNotExist(err) {
		initCmd := exec.CommandContext(ctx, "git", "init")
		initCmd.Dir = absDir
		if err := initCmd.Run(); err != nil {
			fmt.Printf("git init error in %s: %v\n", absDir, err)
			return
		}
	}

	addCmd := exec.CommandContext(ctx, "git", "add", ".")
	addCmd.Dir = absDir
	if err := addCmd.Run(); err != nil {
		fmt.Printf("git add error in %s: %v\n", absDir, err)
		return
	}

	commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	commitCmd.Dir = absDir
	// git commit may return non-zero if there's nothing to commit, so we ignore error
	_ = commitCmd.Run()
}

func getGitLog(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	file := c.QueryParam("file")
	args := []string{"log", "--all", "--pretty=format:%H|%an|%ai|%s", "-n", "50"}
	if file != "" {
		args = append(args, "--", file)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = pagesDir
	output, err := cmd.Output()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	lines := strings.Split(string(output), "\n")
	commits := []CommitInfo{}
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		t, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[2])
		commits = append(commits, CommitInfo{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    t,
			Subject: parts[3],
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"commits": commits})
}

func getGitDiff(c echo.Context) error {
	hash := c.QueryParam("hash")
	if hash == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "hash is required"})
	}
	file := c.QueryParam("file")

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	args := []string{"show", hash}
	if file != "" {
		args = append(args, "--", file)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = pagesDir
	output, err := cmd.Output()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, DiffInfo{Diff: string(output)})
}

func checkoutCommit(c echo.Context) error {
	var req struct {
		Hash string `json:"hash"`
	}
	if err := c.Bind(&req); err != nil {
		return err
	}
	if req.Hash == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "hash is required"})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "checkout", req.Hash)
	cmd.Dir = pagesDir
	err := cmd.Run()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusOK)
}

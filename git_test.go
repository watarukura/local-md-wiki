package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGitTestRepo(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "lwm-git-test-*")
	require.NoError(t, err)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Set git user for commit
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	_ = cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	_ = cmd.Run()

	return tempDir
}

func TestGitHandlers(t *testing.T) {
	// Save original pagesDir and restore after test
	originalPagesDir := pagesDir
	defer func() { pagesDir = originalPagesDir }()

	tempDir := setupGitTestRepo(t)
	defer func() { _ = os.RemoveAll(tempDir) }()
	pagesDir = tempDir

	// Create a file and commit
	err := os.WriteFile(filepath.Join(tempDir, "Test.md"), []byte("# Test Page\nInitial content"), 0644)
	require.NoError(t, err)

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Get first commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = tempDir
	out, err := cmd.Output()
	require.NoError(t, err)
	firstHash := strings.TrimSpace(string(out))

	e := echo.New()

	t.Run("getGitLog", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/git/log", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, getGitLog(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			var res map[string][]CommitInfo
			err := json.Unmarshal(rec.Body.Bytes(), &res)
			assert.NoError(t, err)
			assert.Len(t, res["commits"], 1)
			assert.Equal(t, "Initial commit", res["commits"][0].Subject)
			assert.Equal(t, firstHash, res["commits"][0].Hash)
		}
	})

	// Make another commit
	err = os.WriteFile(filepath.Join(tempDir, "Test.md"), []byte("# Test Page\nModified content"), 0644)
	require.NoError(t, err)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Second commit")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Get second commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = tempDir
	out, err = cmd.Output()
	require.NoError(t, err)
	secondHash := strings.TrimSpace(string(out))

	t.Run("getGitDiff", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/git/diff?hash="+secondHash, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, getGitDiff(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			var res DiffInfo
			err := json.Unmarshal(rec.Body.Bytes(), &res)
			assert.NoError(t, err)
			assert.Contains(t, res.Diff, "Modified content")
		}
	})

	t.Run("getGitDiff_MissingHash", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/git/diff", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, getGitDiff(c)) {
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("checkoutCommit", func(t *testing.T) {
		body := `{"hash":"` + firstHash + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/git/checkout", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, checkoutCommit(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)

			// Verify file content is back to initial
			content, err := os.ReadFile(filepath.Join(tempDir, "Test.md"))
			assert.NoError(t, err)
			assert.Contains(t, string(content), "Initial content")
		}
	})

	t.Run("checkoutCommit_InvalidHash", func(t *testing.T) {
		body := `{"hash":"invalidhash"}`
		req := httptest.NewRequest(http.MethodPost, "/api/git/checkout", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, checkoutCommit(c)) {
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
		}
	})

	t.Run("filtering", func(t *testing.T) {
		// Create another file and commit
		err := os.WriteFile(filepath.Join(tempDir, "Other.md"), []byte("Other"), 0644)
		require.NoError(t, err)
		cmd := exec.Command("git", "add", ".")
		cmd.Dir = tempDir
		err = cmd.Run()
		require.NoError(t, err)
		cmd = exec.Command("git", "commit", "-m", "Other commit")
		cmd.Dir = tempDir
		err = cmd.Run()
		require.NoError(t, err)

		cmd = exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = tempDir
		out, err := cmd.Output()
		require.NoError(t, err)
		headHash := strings.TrimSpace(string(out))

		t.Run("getGitLog_Filter", func(t *testing.T) {
			// Log for Test.md should only have 2 commits (Initial and Second)
			req := httptest.NewRequest(http.MethodGet, "/api/git/log?file=Test.md", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if assert.NoError(t, getGitLog(c)) {
				var res map[string][]CommitInfo
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				assert.NoError(t, err)
				// second commit and initial commit
				assert.Len(t, res["commits"], 2)
				for _, commit := range res["commits"] {
					assert.NotEqual(t, "Other commit", commit.Subject)
				}
			}
		})

		t.Run("getGitDiff_Filter", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/git/diff?hash="+headHash+"&file=Other.md", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if assert.NoError(t, getGitDiff(c)) {
				var res DiffInfo
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				assert.NoError(t, err)
				assert.Contains(t, res.Diff, "diff --git a/Other.md b/Other.md")
			}

			req = httptest.NewRequest(http.MethodGet, "/api/git/diff?hash="+headHash+"&file=Test.md", nil)
			rec = httptest.NewRecorder()
			c = e.NewContext(req, rec)

			if assert.NoError(t, getGitDiff(c)) {
				var res DiffInfo
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				assert.NoError(t, err)
				assert.NotContains(t, res.Diff, "diff --git a/Test.md b/Test.md")
			}
		})
	})
}

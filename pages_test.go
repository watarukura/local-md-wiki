package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupPagesTest(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "lwm-pages-test")
	if err != nil {
		t.Fatal(err)
	}
	originalPagesDir := pagesDir
	pagesDir = tmpDir
	return originalPagesDir
}

func teardownPagesTest(originalPagesDir string) {
	_ = os.RemoveAll(pagesDir)
	pagesDir = originalPagesDir
}

func TestCreatePageHandler(t *testing.T) {
	orig := setupPagesTest(t)
	defer teardownPagesTest(orig)

	e := echo.New()

	t.Run("Success", func(t *testing.T) {
		body := `{"name":"NewPage","markdown":"# New Content","title":"New Title","tags":["test"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/page", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, createPage(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			var res map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &res)
			assert.NoError(t, err)
			assert.Equal(t, true, res["ok"])
			assert.Equal(t, "NewPage.md", res["name"])
		}
	})

	t.Run("Conflict", func(t *testing.T) {
		// Create first
		err := os.WriteFile(filepath.Join(pagesDir, "Existing.md"), []byte("content"), 0644)
		assert.NoError(t, err)

		body := `{"name":"Existing.md","markdown":"content"}`
		req := httptest.NewRequest(http.MethodPost, "/api/page", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, createPage(c)) {
			assert.Equal(t, http.StatusConflict, rec.Code)
		}
	})

	t.Run("InvalidName", func(t *testing.T) {
		body := `{"name":"../invalid","markdown":"content"}`
		req := httptest.NewRequest(http.MethodPost, "/api/page", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, createPage(c)) {
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("BindError", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/page", strings.NewReader("invalid json"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.Error(t, createPage(c))
	})
}

func TestGetPageHandler(t *testing.T) {
	orig := setupPagesTest(t)
	defer teardownPagesTest(orig)
	e := echo.New()
	err := os.WriteFile(filepath.Join(pagesDir, "Home.md"), []byte("---\ntitle: Home\n---\n# Welcome"), 0644)
	assert.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/page?name=Home.md", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, getPage(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			var res PageResponse
			err := json.Unmarshal(rec.Body.Bytes(), &res)
			assert.NoError(t, err)
			assert.Equal(t, "Home.md", res.Name)
			assert.Contains(t, res.HTML, "Welcome")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/page?name=NonExistent.md", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, getPage(c)) {
			assert.Equal(t, http.StatusNotFound, rec.Code)
		}
	})

	t.Run("EmptyNameDefaultsToHome", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/page", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, getPage(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})
}

func TestUpdatePageHandler(t *testing.T) {
	orig := setupPagesTest(t)
	defer teardownPagesTest(orig)
	e := echo.New()
	err := os.WriteFile(filepath.Join(pagesDir, "ToUpdate.md"), []byte("---\ntitle: Old\ncreated_at: 2024-01-01T00:00:00Z\n---\nOld Content"), 0644)
	assert.NoError(t, err)

	t.Run("SuccessUpdate", func(t *testing.T) {
		body := `{"name":"ToUpdate.md","markdown":"# New Content"}`
		req := httptest.NewRequest(http.MethodPut, "/api/page", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, updatePage(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)

			// Verify it merged frontmatter
			content, _ := os.ReadFile(filepath.Join(pagesDir, "ToUpdate.md"))
			assert.Contains(t, string(content), "title: Old")
			assert.Contains(t, string(content), "created_at: \"2024-01-01T00:00:00Z\"")
			assert.Contains(t, string(content), "New Content")
		}
	})

	t.Run("BindError", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/page", strings.NewReader("invalid json"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.Error(t, updatePage(c))
	})
}

func TestListPagesHandler(t *testing.T) {
	orig := setupPagesTest(t)
	defer teardownPagesTest(orig)

	e := echo.New()
	err1 := os.WriteFile(filepath.Join(pagesDir, "Page1.md"), []byte("---\ntitle: Title1\n---\nContent1"), 0644)
	assert.NoError(t, err1)
	err2 := os.WriteFile(filepath.Join(pagesDir, "Page2.md"), []byte("Content2"), 0644)
	assert.NoError(t, err2)

	req := httptest.NewRequest(http.MethodGet, "/api/pages", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if assert.NoError(t, listPages(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var res map[string][]PageInfo
		err := json.Unmarshal(rec.Body.Bytes(), &res)
		assert.NoError(t, err)
		assert.Len(t, res["pages"], 2)
	}
}

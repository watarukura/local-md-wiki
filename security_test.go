package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestUploadFile_PathTraversal(t *testing.T) {
	e := echo.New()

	// Prepare a multipart form with a forbidden extension
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.exe")
	assert.NoError(t, err)
	_, err = io.WriteString(part, "forbidden content")
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	uploadsDir = "test_uploads"
	_ = os.MkdirAll(uploadsDir, 0755)
	defer func() { _ = os.RemoveAll(uploadsDir) }()

	if assert.NoError(t, uploadFile(c)) {
		// Should be BadRequest due to forbidden extension
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var res map[string]string
		err = json.Unmarshal(rec.Body.Bytes(), &res)
		assert.NoError(t, err)
		assert.Equal(t, "file type not allowed", res["error"])
	}
}

func TestSearchPages_InformationDisclosure_Fixed(t *testing.T) {
	e := echo.New()

	// Ensure pagesDir exists
	pagesDir = "test_pages"
	_ = os.MkdirAll(pagesDir, 0755)
	defer func() { _ = os.RemoveAll(pagesDir) }()

	// Create a secret file that is NOT a markdown file
	secretFile := filepath.Join(pagesDir, ".env")
	err := os.WriteFile(secretFile, []byte("SECRET_TOKEN=mysecret123"), 0644)
	assert.NoError(t, err)

	// Create a valid markdown file with search content
	mdFile := filepath.Join(pagesDir, "valid.md")
	err = os.WriteFile(mdFile, []byte("this is a valid markdown with mysecret content"), 0644)
	assert.NoError(t, err)

	// Search for the secret content
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=mysecret", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if assert.NoError(t, searchPages(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)

		var results []SearchResult
		err := json.Unmarshal(rec.Body.Bytes(), &results)
		assert.NoError(t, err)

		foundEnv := false
		foundMd := false
		for _, res := range results {
			if res.File == ".env" {
				foundEnv = true
			}
			if res.File == "valid.md" {
				foundMd = true
			}
		}

		if foundEnv {
			t.Errorf("Vulnerability still exists: searchPages exposed secret information from .env file")
		}
		if !foundMd {
			t.Errorf("Search failed: valid.md should have been found")
		}
	}
}

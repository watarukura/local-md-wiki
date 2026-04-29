package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestUploadFileHandler(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lwm-upload-test")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	originalUploadsDir := uploadsDir
	uploadsDir = tmpDir
	defer func() { uploadsDir = originalUploadsDir }()

	e := echo.New()

	t.Run("Success", func(t *testing.T) {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "test.png")
		assert.NoError(t, err)
		_, err = part.Write([]byte("fake image content"))
		assert.NoError(t, err)
		err = writer.Close()
		assert.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
		req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, uploadFile(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("InvalidExtension", func(t *testing.T) {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "test.exe")
		assert.NoError(t, err)
		_, err = part.Write([]byte("fake binary"))
		assert.NoError(t, err)
		err = writer.Close()
		assert.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
		req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, uploadFile(c)) {
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("NoFile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/upload", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.Error(t, uploadFile(c))
	})
}

func TestListMarkdownFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lwm-list-test")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	err = os.MkdirAll(filepath.Join(tmpDir, "sub"), 0755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "top.md"), []byte("top"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "sub", "bottom.md"), []byte("bottom"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "ignore.txt"), []byte("ignore"), 0644)
	assert.NoError(t, err)

	files, err := listMarkdownFiles(tmpDir, "")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"top.md", "sub/bottom.md"}, files)

	// Test error path
	_, err = listMarkdownFiles("/non/existent/path", "")
	assert.Error(t, err)
}

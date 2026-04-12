package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestListPages(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/pages", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if assert.NoError(t, listPages(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var res map[string][]PageInfo
		err := json.Unmarshal(rec.Body.Bytes(), &res)
		assert.NoError(t, err)
		assert.NotEmpty(t, res["pages"])
	}
}

func TestGetPage(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/page?name=Home.md", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if assert.NoError(t, getPage(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var res PageResponse
		err := json.Unmarshal(rec.Body.Bytes(), &res)
		assert.NoError(t, err)
		assert.Equal(t, "Home.md", res.Name)
	}
}

func TestUpdatePage(t *testing.T) {
	e := echo.New()
	body := `{"name":"Home.md","markdown":"no frontmatter content"}`
	req := httptest.NewRequest(http.MethodPost, "/api/page", strings.NewReader(body)) // test create via POST
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	_ = e.NewContext(req, rec)

	// Since Home.md already exists, we expect StatusConflict if we use createPage
	// But let's test updatePage which uses PUT
	req = httptest.NewRequest(http.MethodPut, "/api/page", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if assert.NoError(t, updatePage(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var res map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &res)
		assert.NoError(t, err)
		assert.Equal(t, true, res["ok"])
		// Verify backlinks and twoHop are not nil (null in JSON)
		assert.NotNil(t, res["backlinks"])
		assert.NotNil(t, res["twoHop"])
		// Check they are indeed empty arrays
		assert.IsType(t, []interface{}{}, res["backlinks"])
		assert.IsType(t, []interface{}{}, res["twoHop"])
	}
}

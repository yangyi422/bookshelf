package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWebStaticFilesAndSPAFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	webDir := t.TempDir()
	assetsDir := filepath.Join(webDir, "assets")
	if err := os.Mkdir(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	index := "<!doctype html><html><body>bookshelf-spa</body></html>"
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), []byte("window.bookshelf = true"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	mountWeb(r, webDir)

	tests := []struct {
		path        string
		status      int
		contentType string
		contains    string
		notContains string
	}{
		{path: "/", status: http.StatusOK, contentType: "text/html", contains: "bookshelf-spa"},
		{path: "/setup", status: http.StatusOK, contentType: "text/html", contains: "bookshelf-spa"},
		{path: "/assets/app.js", status: http.StatusOK, contains: "window.bookshelf"},
		{path: "/api/v1/not-exist", status: http.StatusNotFound, contentType: "application/json", notContains: "bookshelf-spa"},
		{path: "/opds/not-exist", status: http.StatusNotFound, contentType: "application/json", notContains: "bookshelf-spa"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if w.Code != tt.status {
				t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
			}
			if tt.contentType != "" && !strings.HasPrefix(w.Header().Get("Content-Type"), tt.contentType) {
				t.Fatalf("Content-Type = %q", w.Header().Get("Content-Type"))
			}
			if tt.contains != "" && !strings.Contains(w.Body.String(), tt.contains) {
				t.Fatalf("body = %q", w.Body.String())
			}
			if tt.notContains != "" && strings.Contains(w.Body.String(), tt.notContains) {
				t.Fatalf("unexpected SPA response: %q", w.Body.String())
			}
		})
	}
}

func TestMissingAssetDoesNotExposeFilesystemPath(t *testing.T) {
	r := gin.New()
	webDir := t.TempDir()
	mountWeb(r, webDir)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), webDir) {
		t.Fatalf("response exposed web directory: %q", w.Body.String())
	}
}

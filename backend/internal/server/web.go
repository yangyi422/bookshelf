package server

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func mountWeb(r *gin.Engine, webDir string) {
	webDir = strings.TrimSpace(webDir)
	if webDir == "" {
		return
	}

	indexPath := filepath.Join(webDir, "index.html")
	r.GET("/", serveWebFile(indexPath))
	r.GET("/assets/*filepath", func(c *gin.Context) {
		rel := strings.TrimPrefix(c.Param("filepath"), "/")
		if rel == "" || filepath.Clean(rel) != rel || strings.Contains(rel, "..") {
			c.Status(http.StatusNotFound)
			return
		}
		serveFile(c, filepath.Join(webDir, "assets", rel))
	})
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if c.Request.Method != http.MethodGet || isServerPath(path) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		serveFile(c, indexPath)
	})
}

func serveWebFile(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		serveFile(c, path)
	}
}

func serveFile(c *gin.Context, path string) {
	f, err := os.Open(path)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		c.Status(http.StatusNotFound)
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(info.Name()))
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, f)
}

func isServerPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/") ||
		path == "/opds" || strings.HasPrefix(path, "/opds/") ||
		path == "/opensearch.xml" || strings.HasPrefix(path, "/opensearch.xml/")
}

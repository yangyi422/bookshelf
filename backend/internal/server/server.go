package server

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"bookshelf/internal/auth"
	"bookshelf/internal/backup"
	"bookshelf/internal/book"
	"bookshelf/internal/catalog"
	"bookshelf/internal/config"
	appmw "bookshelf/internal/middleware"
	"bookshelf/internal/opds"
	"bookshelf/internal/proxy"
	"bookshelf/internal/scanner"
	"bookshelf/internal/settings"
	"bookshelf/internal/storage"
	appsystem "bookshelf/internal/system"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, store *storage.Storage, authService *auth.Service, settingsService *settings.Service, resolver *proxy.Resolver) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	_ = r.SetTrustedProxies(cfg.TrustedProxies)
	r.Use(gin.Recovery(), appmw.SecurityHeaders(), func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	})
	r.GET("/api/v1/system/health", func(c *gin.Context) {
		var one int
		if err := db.Raw("SELECT 1").Scan(&one).Error; err != nil || one != 1 {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "database": "error"})
			return
		}
		if _, err := os.Stat(store.Root()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "storage": "error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "ok", "storage": "ok"})
	})
	backupService := backup.New(db, store, cfg.BackupRetentionDays)
	scanService := scanner.New(db, store)
	systemHandler := appsystem.New(db, store, backupService, scanService, settingsService, resolver)
	r.GET("/api/v1/setup/status", systemHandler.SetupStatus)
	r.POST("/api/v1/setup", systemHandler.Initialize)
	h := auth.NewHandler(authService, cfg.Environment == "production", cfg.SessionTTL)
	a := r.Group("/api/v1/auth")
	a.POST("/login", appmw.LoginRateLimit(10, time.Minute), h.Login)
	protected := a.Group("")
	protected.Use(h.RequireUser())
	protected.POST("/logout", h.Logout)
	protected.GET("/me", h.Me)
	protected.POST("/change-password", h.ChangePassword)
	api := r.Group("/api/v1")
	api.Use(h.RequireUser())
	bookService := book.NewService(db, store, cfg.MaxUploadBytes)
	bh := book.NewHandler(bookService, db, cfg.MaxUploadBytes)
	api.GET("/books", bh.List)
	api.GET("/books/trash", bh.Trash)
	api.POST("/books", bh.Create)
	api.GET("/books/:id", bh.Get)
	api.PUT("/books/:id", bh.Update)
	api.DELETE("/books/:id", bh.Delete)
	api.POST("/books/:id/restore", bh.Restore)
	api.POST("/books/:id/cover", bh.SetCover)
	api.GET("/books/:id/cover", bh.Cover)
	api.POST("/books/:id/files", appmw.LoginRateLimit(30, time.Minute), bh.AddFile)
	api.GET("/books/:id/files", bh.Files)
	api.DELETE("/books/:id/files/:fileId", bh.DeleteFile)
	api.GET("/books/:id/files/:fileId/download", bh.Download)
	api.POST("/imports", appmw.LoginRateLimit(30, time.Minute), bh.Import)
	api.GET("/imports", bh.Imports)
	api.GET("/imports/:id", bh.ImportGet)
	ch := catalog.NewHandler(db)
	api.GET("/authors", ch.Authors)
	api.POST("/authors", ch.CreateAuthor)
	api.PUT("/authors/:id", ch.UpdateAuthor)
	api.DELETE("/authors/:id", ch.DeleteAuthor)
	api.GET("/tags", ch.Tags)
	api.POST("/tags", ch.CreateTag)
	api.PUT("/tags/:id", ch.UpdateTag)
	api.DELETE("/tags/:id", ch.DeleteTag)
	api.GET("/system/info", systemHandler.Info)
	api.POST("/system/backups", systemHandler.Backup)
	api.GET("/system/backups", systemHandler.Backups)
	api.GET("/system/backups/:id/validate", systemHandler.ValidateBackup)
	api.POST("/system/scan", systemHandler.Scan)
	api.GET("/system/scan/status", systemHandler.ScanStatus)
	api.GET("/system/manifest", systemHandler.Manifest)
	api.GET("/system/opds", systemHandler.OPDSSettings)
	api.PUT("/system/opds", systemHandler.UpdateOPDSSettings)
	api.POST("/system/opds/test", systemHandler.TestOPDS)
	oh := opds.New(db, bookService, settingsService, resolver)
	opdsCatalog := r.Group("")
	opdsCatalog.Use(oh.BasicAuth())
	opdsCatalog.GET("/opds", oh.Root)
	opdsCatalog.GET("/opds/recent", oh.Recent)
	opdsCatalog.GET("/opds/all", oh.All)
	opdsCatalog.GET("/opds/authors", oh.Authors)
	opdsCatalog.GET("/opds/authors/:id", oh.AuthorBooks)
	opdsCatalog.GET("/opds/tags", oh.Tags)
	opdsCatalog.GET("/opds/tags/:id", oh.TagBooks)
	opdsCatalog.GET("/opds/formats", oh.Formats)
	opdsCatalog.GET("/opds/formats/:format", oh.FormatBooks)
	opdsCatalog.GET("/opds/search", oh.Search)
	opdsCatalog.GET("/opds/books/:id", oh.Book)
	opdsCatalog.GET("/opds/books/:id/cover", oh.Cover)
	opdsCatalog.GET("/opds/books/:id/files/:fileId", oh.Download)
	opdsCatalog.GET("/opensearch.xml", oh.OpenSearch)
	return r
}

package system

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"bookshelf/internal/auth"
	"bookshelf/internal/backup"
	"bookshelf/internal/database"
	"bookshelf/internal/proxy"
	"bookshelf/internal/scanner"
	appsettings "bookshelf/internal/settings"
	"bookshelf/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	db       *gorm.DB
	store    *storage.Storage
	backup   *backup.Service
	scanner  *scanner.Service
	settings *appsettings.Service
	proxy    *proxy.Resolver
}

func New(db *gorm.DB, store *storage.Storage, b *backup.Service, s *scanner.Service, settings *appsettings.Service, resolver *proxy.Resolver) *Handler {
	return &Handler{db: db, store: store, backup: b, scanner: s, settings: settings, proxy: resolver}
}
func (h *Handler) Info(c *gin.Context) {
	var books, files int64
	h.db.Model(&database.Book{}).Where("deleted_at IS NULL").Count(&books)
	h.db.Model(&database.BookFile{}).Count(&files)
	view, _ := h.settings.View()
	opdsURL := ""
	if view.OPDSEnabled {
		opdsURL = h.proxy.Origin(c.Request, view.PublicBaseURL) + "/opds"
	}
	c.JSON(200, gin.H{"books": books, "files": files, "data_dir": h.store.Root(), "opds_url": opdsURL})
}

func (h *Handler) SetupStatus(c *gin.Context) {
	initialized, err := h.settings.Initialized()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read initialization state"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"initialized": initialized})
}

func (h *Handler) Initialize(c *gin.Context) {
	var in appsettings.SetupInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid initialization request"})
		return
	}
	view, err := h.settings.Initialize(in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, view)
}

func (h *Handler) OPDSSettings(c *gin.Context) {
	view, err := h.settings.View()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read OPDS settings"})
		return
	}
	if view.OPDSEnabled {
		view.OPDSURL = h.proxy.Origin(c.Request, view.PublicBaseURL) + "/opds"
	}
	c.JSON(http.StatusOK, view)
}

func (h *Handler) UpdateOPDSSettings(c *gin.Context) {
	u := auth.CurrentUser(c)
	if u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "administrator access required"})
		return
	}
	var in appsettings.Update
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid OPDS settings"})
		return
	}
	view, err := h.settings.Update(in, u)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if view.OPDSEnabled {
		view.OPDSURL = h.proxy.Origin(c.Request, view.PublicBaseURL) + "/opds"
	}
	h.audit(c, "opds.settings.update", "system", "")
	c.JSON(http.StatusOK, view)
}

func (h *Handler) TestOPDS(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !h.settings.VerifyCredentials(strings.TrimSpace(in.Username), in.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OPDS connectivity test failed"})
		return
	}
	view, _ := h.settings.View()
	c.JSON(http.StatusOK, gin.H{"ok": true, "opds_url": h.proxy.Origin(c.Request, view.PublicBaseURL) + "/opds"})
}
func (h *Handler) Backup(c *gin.Context) {
	r, err := h.backup.Create()
	if err != nil {
		slog.Error("backup failed", "error", err)
		c.JSON(500, gin.H{"error": "backup failed"})
		return
	}
	h.audit(c, "backup.create", "backup", r.ID)
	c.JSON(201, r)
}
func (h *Handler) Backups(c *gin.Context) {
	rows, err := h.backup.List()
	if err != nil {
		c.JSON(500, gin.H{"error": "could not list backups"})
		return
	}
	c.JSON(200, rows)
}
func (h *Handler) ValidateBackup(c *gin.Context) {
	r, err := h.backup.Validate(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"valid": true, "backup": r})
}
func (h *Handler) Scan(c *gin.Context) {
	r, err := h.scanner.Scan()
	if err != nil {
		slog.Error("scan failed", "error", err)
		c.JSON(500, gin.H{"error": "scan failed"})
		return
	}
	h.audit(c, "scan.run", "system", "")
	c.JSON(200, r)
}
func (h *Handler) ScanStatus(c *gin.Context) { c.JSON(200, h.scanner.Last()) }
func (h *Handler) Manifest(c *gin.Context) {
	m, err := h.scanner.ExportManifest()
	if err != nil {
		slog.Error("manifest export failed", "error", err)
		c.JSON(500, gin.H{"error": "manifest export failed"})
		return
	}
	h.audit(c, "manifest.export", "system", "")
	c.JSON(200, m)
}
func (h *Handler) audit(c *gin.Context, action, kind, id string) {
	u := auth.CurrentUser(c)
	if err := h.db.Create(&database.AuditLog{ID: uuid.NewString(), UserID: u.ID, Action: action, ResourceType: kind, ResourceID: id, RemoteIP: c.ClientIP(), CreatedAt: time.Now().UTC()}).Error; err != nil {
		slog.Error("audit log write failed", "action", action, "error", err)
	}
}

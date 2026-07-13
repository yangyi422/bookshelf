package system

import (
	"log/slog"
	"time"

	"bookshelf/internal/auth"
	"bookshelf/internal/backup"
	"bookshelf/internal/database"
	"bookshelf/internal/scanner"
	"bookshelf/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	db      *gorm.DB
	store   *storage.Storage
	backup  *backup.Service
	scanner *scanner.Service
	opdsURL string
}

func New(db *gorm.DB, store *storage.Storage, b *backup.Service, s *scanner.Service, opdsURL string) *Handler {
	return &Handler{db: db, store: store, backup: b, scanner: s, opdsURL: opdsURL}
}
func (h *Handler) Info(c *gin.Context) {
	var books, files int64
	h.db.Model(&database.Book{}).Where("deleted_at IS NULL").Count(&books)
	h.db.Model(&database.BookFile{}).Count(&files)
	c.JSON(200, gin.H{"books": books, "files": files, "data_dir": h.store.Root(), "opds_url": h.opdsURL})
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

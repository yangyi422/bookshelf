package book

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"bookshelf/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	service   *Service
	db        *gorm.DB
	maxUpload int64
}

func NewHandler(s *Service, db *gorm.DB, max int64) *Handler {
	return &Handler{service: s, db: db, maxUpload: max}
}
func (h *Handler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	items, total, err := h.service.List(ListOptions{Keyword: c.Query("keyword"), AuthorID: c.Query("author_id"), TagID: c.Query("tag_id"), Format: c.Query("format"), ReadingStatus: c.Query("reading_status"), Sort: c.Query("sort"), Order: c.Query("order"), Page: page, Size: size})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}
func (h *Handler) Trash(c *gin.Context) {
	items, err := h.service.Trash()
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, items)
}
func (h *Handler) Create(c *gin.Context) {
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid book data"})
		return
	}
	d, err := h.service.Create(in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(201, d)
}
func (h *Handler) Get(c *gin.Context) {
	d, err := h.service.Get(c.Param("id"), false)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, d)
}
func (h *Handler) Update(c *gin.Context) {
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid book data"})
		return
	}
	d, err := h.service.Update(c.Param("id"), in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, d)
}
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
func (h *Handler) Restore(c *gin.Context) {
	if err := h.service.Restore(c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
func (h *Handler) AddFile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUpload+1024*1024)
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "multipart field file is required"})
		return
	}
	f, err := h.service.AddFile(c.Param("id"), fh)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(201, f)
}
func (h *Handler) Files(c *gin.Context) {
	d, err := h.service.Get(c.Param("id"), false)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, d.Files)
}
func (h *Handler) SetCover(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 11<<20)
	fh, err := c.FormFile("cover")
	if err != nil {
		c.JSON(400, gin.H{"error": "multipart field cover is required"})
		return
	}
	if err = h.service.SetCover(c.Param("id"), fh); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
func (h *Handler) Cover(c *gin.Context) {
	mime, f, err := h.service.Cover(c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.DataFromReader(200, stat.Size(), mime, f, nil)
}
func (h *Handler) DeleteFile(c *gin.Context) {
	if err := h.service.DeleteFile(c.Param("id"), c.Param("fileId")); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
func (h *Handler) Download(c *gin.Context) {
	f, handle, err := h.service.File(c.Param("id"), c.Param("fileId"))
	if err != nil {
		fail(c, err)
		return
	}
	defer handle.Close()
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", urlEncode(f.OriginalName)))
	c.Header("Accept-Ranges", "bytes")
	c.DataFromReader(200, f.FileSize, f.MIMEType, handle, nil)
}
func (h *Handler) Import(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUpload+1024*1024)
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "multipart field file is required"})
		return
	}
	job := database.ImportJob{ID: uuid.NewString(), OriginalName: fh.Filename, Status: "processing"}
	if err := h.db.Create(&job).Error; err != nil {
		fail(c, err)
		return
	}
	d, err := h.service.Import(fh)
	if err != nil {
		h.db.Model(&job).Updates(map[string]any{"status": "failed", "error_message": err.Error(), "updated_at": time.Now()})
		fail(c, err)
		return
	}
	h.db.Model(&job).Updates(map[string]any{"status": "success", "updated_at": time.Now()})
	job.Status = "success"
	job.UpdatedAt = time.Now()
	c.JSON(201, gin.H{"job": job, "book": d})
}
func (h *Handler) Imports(c *gin.Context) {
	var jobs []database.ImportJob
	if err := h.db.Order("created_at DESC").Limit(100).Find(&jobs).Error; err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, jobs)
}
func (h *Handler) ImportGet(c *gin.Context) {
	var j database.ImportJob
	if err := h.db.First(&j, "id=?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "import job not found"})
		return
	}
	c.JSON(200, j)
}
func fail(c *gin.Context, err error) {
	status := 400
	if errors.Is(err, ErrNotFound) {
		status = 404
	}
	if errors.Is(err, ErrDuplicate) {
		status = 409
	}
	c.JSON(status, gin.H{"error": err.Error()})
}
func urlEncode(v string) string {
	out := ""
	for _, b := range []byte(v) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '.' || b == '-' || b == '_' {
			out += string(b)
		} else {
			out += fmt.Sprintf("%%%02X", b)
		}
	}
	return out
}

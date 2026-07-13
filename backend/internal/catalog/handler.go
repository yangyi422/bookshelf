package catalog

import (
	"net/http"
	"strings"

	"bookshelf/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct{ db *gorm.DB }

func NewHandler(db *gorm.DB) *Handler { return &Handler{db: db} }
func (h *Handler) Authors(c *gin.Context) {
	var rows []database.Author
	if err := h.db.Order("sort_name,name").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not list authors"})
		return
	}
	c.JSON(200, rows)
}
func (h *Handler) CreateAuthor(c *gin.Context) {
	var in struct {
		Name     string `json:"name"`
		SortName string `json:"sort_name"`
	}
	if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.Name) == "" {
		c.JSON(400, gin.H{"error": "name is required"})
		return
	}
	a := database.Author{ID: uuid.NewString(), Name: strings.TrimSpace(in.Name), SortName: strings.TrimSpace(in.SortName)}
	if err := h.db.Create(&a).Error; err != nil {
		c.JSON(409, gin.H{"error": "could not create author"})
		return
	}
	c.JSON(201, a)
}
func (h *Handler) UpdateAuthor(c *gin.Context) {
	var in struct {
		Name     string `json:"name"`
		SortName string `json:"sort_name"`
	}
	if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.Name) == "" {
		c.JSON(400, gin.H{"error": "name is required"})
		return
	}
	r := h.db.Model(&database.Author{}).Where("id=?", c.Param("id")).Updates(map[string]any{"name": strings.TrimSpace(in.Name), "sort_name": strings.TrimSpace(in.SortName)})
	if r.Error != nil {
		c.JSON(409, gin.H{"error": "could not update author"})
		return
	}
	if r.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "author not found"})
		return
	}
	c.Status(204)
}
func (h *Handler) DeleteAuthor(c *gin.Context) {
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("author_id=?", c.Param("id")).Delete(&database.BookAuthor{}).Error; err != nil {
			return err
		}
		r := tx.Delete(&database.Author{}, "id=?", c.Param("id"))
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err == gorm.ErrRecordNotFound {
		c.JSON(404, gin.H{"error": "author not found"})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"error": "could not delete author"})
		return
	}
	c.Status(204)
}
func (h *Handler) Tags(c *gin.Context) {
	var rows []database.Tag
	if err := h.db.Order("name").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not list tags"})
		return
	}
	c.JSON(200, rows)
}
func (h *Handler) CreateTag(c *gin.Context) {
	var in struct {
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.Name) == "" {
		c.JSON(400, gin.H{"error": "name is required"})
		return
	}
	t := database.Tag{ID: uuid.NewString(), Name: strings.TrimSpace(in.Name)}
	if err := h.db.Create(&t).Error; err != nil {
		c.JSON(409, gin.H{"error": "tag name already exists"})
		return
	}
	c.JSON(201, t)
}
func (h *Handler) UpdateTag(c *gin.Context) {
	var in struct {
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.Name) == "" {
		c.JSON(400, gin.H{"error": "name is required"})
		return
	}
	r := h.db.Model(&database.Tag{}).Where("id=?", c.Param("id")).Update("name", strings.TrimSpace(in.Name))
	if r.Error != nil {
		c.JSON(409, gin.H{"error": "tag name already exists"})
		return
	}
	if r.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "tag not found"})
		return
	}
	c.Status(204)
}
func (h *Handler) DeleteTag(c *gin.Context) {
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id=?", c.Param("id")).Delete(&database.BookTag{}).Error; err != nil {
			return err
		}
		r := tx.Delete(&database.Tag{}, "id=?", c.Param("id"))
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err == gorm.ErrRecordNotFound {
		c.JSON(404, gin.H{"error": "tag not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete tag"})
		return
	}
	c.Status(204)
}

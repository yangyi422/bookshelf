package opds

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bookshelf/internal/book"
	"bookshelf/internal/database"
	"bookshelf/internal/proxy"
	appsettings "bookshelf/internal/settings"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Handler struct {
	db       *gorm.DB
	books    *book.Service
	settings *appsettings.Service
	proxy    *proxy.Resolver
}

const settingKey = "opds_setting"

func New(db *gorm.DB, books *book.Service, settings *appsettings.Service, resolver *proxy.Resolver) *Handler {
	return &Handler{db: db, books: books, settings: settings, proxy: resolver}
}
func (h *Handler) BasicAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		setting, err := h.settings.Current()
		if err != nil || !setting.OPDSEnabled || setting.OPDSAccessMode == appsettings.ModeDisabled {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if setting.OPDSAccessMode == appsettings.ModeHTTPSOnly && !h.proxy.HTTPS(c.Request) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "OPDS requires HTTPS"})
			return
		}
		user, pass, ok := c.Request.BasicAuth()
		if !ok || !secureEqual(user, setting.OPDSUsername) || bcrypt.CompareHashAndPassword([]byte(setting.OPDSPasswordHash), []byte(pass)) != nil {
			c.Header("WWW-Authenticate", `Basic realm="Bookshelf OPDS", charset="UTF-8"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set(settingKey, setting)
		c.Next()
	}
}
func secureEqual(a, b string) bool {
	ah := sha256.Sum256([]byte(a))
	bh := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
}
func (h *Handler) URL(c *gin.Context, p string) string {
	setting, _ := c.Get(settingKey)
	row, _ := setting.(database.SystemSetting)
	return strings.TrimRight(h.proxy.Origin(c.Request, row.PublicBaseURL), "/") + "/" + strings.TrimLeft(p, "/")
}
func (h *Handler) Root(c *gin.Context) {
	now := time.Now().UTC().Format(time.RFC3339)
	f := newFeed(h.URL(c, "/opds"), "Bookshelf", now)
	f.Links = h.commonLinks(c, "/opds", navType)
	items := []struct{ title, path string }{{"最近添加", "/opds/recent"}, {"全部书籍", "/opds/all"}, {"作者", "/opds/authors"}, {"标签", "/opds/tags"}, {"格式", "/opds/formats"}}
	for _, it := range items {
		f.Entries = append(f.Entries, Entry{ID: h.URL(c, it.path), Title: it.title, Updated: now, Content: &Content{Type: "text", Text: it.title}, Links: []Link{{Rel: "subsection", Href: h.URL(c, it.path), Type: navType}}})
	}
	h.xml(c, navType, f)
}
func (h *Handler) Recent(c *gin.Context) {
	h.acquisition(c, "最近添加", book.ListOptions{Sort: "created_at", Order: "desc"})
}
func (h *Handler) All(c *gin.Context) {
	h.acquisition(c, "全部书籍", book.ListOptions{Sort: c.DefaultQuery("sort", "title"), Order: c.DefaultQuery("order", "asc")})
}
func (h *Handler) Search(c *gin.Context) {
	h.acquisition(c, "搜索: "+c.Query("q"), book.ListOptions{Keyword: c.Query("q"), Sort: "title", Order: "asc"})
}
func (h *Handler) AuthorBooks(c *gin.Context) {
	var a database.Author
	if h.db.First(&a, "id=?", c.Param("id")).Error != nil {
		c.Status(404)
		return
	}
	h.acquisition(c, a.Name, book.ListOptions{AuthorID: a.ID, Sort: "title", Order: "asc"})
}
func (h *Handler) TagBooks(c *gin.Context) {
	var t database.Tag
	if h.db.First(&t, "id=?", c.Param("id")).Error != nil {
		c.Status(404)
		return
	}
	h.acquisition(c, t.Name, book.ListOptions{TagID: t.ID, Sort: "title", Order: "asc"})
}
func (h *Handler) FormatBooks(c *gin.Context) {
	format := strings.ToLower(c.Param("format"))
	allowed := map[string]bool{"epub": true, "pdf": true, "mobi": true, "azw3": true, "txt": true}
	if !allowed[format] {
		c.Status(404)
		return
	}
	h.acquisition(c, strings.ToUpper(format), book.ListOptions{Format: format, Sort: "title", Order: "asc"})
}
func (h *Handler) acquisition(c *gin.Context, title string, o book.ListOptions) {
	page, size := paging(c)
	o.Page = page
	o.Size = size
	items, total, err := h.books.List(o)
	if err != nil {
		c.Status(500)
		return
	}
	self := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		self += "?" + c.Request.URL.RawQuery
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if len(items) > 0 {
		now = items[0].UpdatedAt.UTC().Format(time.RFC3339)
	}
	f := newFeed(h.URL(c, self), title, now)
	f.Links = h.commonLinks(c, self, acqType)
	f.Links = append(f.Links, Link{Rel: "start", Href: h.URL(c, "/opds"), Type: navType})
	if page > 1 {
		f.Links = append(f.Links, Link{Rel: "previous", Href: h.pageURL(c, page-1, size), Type: acqType})
	}
	if int64(page*size) < total {
		f.Links = append(f.Links, Link{Rel: "next", Href: h.pageURL(c, page+1, size), Type: acqType})
	}
	for _, d := range items {
		f.Entries = append(f.Entries, h.bookEntry(c, d))
	}
	h.xml(c, acqType, f)
}
func (h *Handler) Book(c *gin.Context) {
	d, err := h.books.Get(c.Param("id"), false)
	if err != nil {
		c.Status(404)
		return
	}
	f := newFeed(h.URL(c, "/opds/books/"+d.ID), d.Title, d.UpdatedAt.UTC().Format(time.RFC3339))
	f.Links = h.commonLinks(c, "/opds/books/"+d.ID, acqType)
	f.Entries = []Entry{h.bookEntry(c, d)}
	h.xml(c, acqType, f)
}
func (h *Handler) bookEntry(c *gin.Context, d book.Detail) Entry {
	e := Entry{ID: "urn:uuid:" + d.ID, Title: d.Title, Updated: d.UpdatedAt.UTC().Format(time.RFC3339), Summary: d.Description, Language: d.Language, Publisher: d.Publisher, Identifier: d.ISBN}
	for _, a := range d.Authors {
		e.Authors = append(e.Authors, Person{Name: a.Name})
	}
	for _, t := range d.Tags {
		e.Categories = append(e.Categories, Category{Term: t.ID, Label: t.Name})
	}
	if d.CoverPath != "" {
		href := h.URL(c, "/opds/books/"+d.ID+"/cover")
		coverType := "image/jpeg"
		if strings.HasSuffix(d.CoverPath, ".png") {
			coverType = "image/png"
		} else if strings.HasSuffix(d.CoverPath, ".webp") {
			coverType = "image/webp"
		}
		e.Links = append(e.Links, Link{Rel: "http://opds-spec.org/image", Href: href, Type: coverType}, Link{Rel: "http://opds-spec.org/image/thumbnail", Href: href, Type: coverType})
	}
	for _, f := range d.Files {
		e.Links = append(e.Links, Link{Rel: "http://opds-spec.org/acquisition", Href: h.URL(c, "/opds/books/"+d.ID+"/files/"+f.ID), Type: f.MIMEType, Title: f.Format})
	}
	e.Links = append(e.Links, Link{Rel: "alternate", Href: h.URL(c, "/opds/books/"+d.ID), Type: acqType})
	return e
}
func (h *Handler) Authors(c *gin.Context) {
	var rows []database.Author
	h.db.Order("sort_name,name").Find(&rows)
	h.navigationRows(c, "作者", func() []Entry {
		out := []Entry{}
		now := time.Now().UTC().Format(time.RFC3339)
		for _, a := range rows {
			p := "/opds/authors/" + url.PathEscape(a.ID)
			out = append(out, Entry{ID: h.URL(c, p), Title: a.Name, Updated: now, Links: []Link{{Rel: "subsection", Href: h.URL(c, p), Type: acqType}}})
		}
		return out
	}())
}
func (h *Handler) Tags(c *gin.Context) {
	var rows []database.Tag
	h.db.Order("name").Find(&rows)
	h.navigationRows(c, "标签", func() []Entry {
		out := []Entry{}
		now := time.Now().UTC().Format(time.RFC3339)
		for _, t := range rows {
			p := "/opds/tags/" + url.PathEscape(t.ID)
			out = append(out, Entry{ID: h.URL(c, p), Title: t.Name, Updated: now, Links: []Link{{Rel: "subsection", Href: h.URL(c, p), Type: acqType}}})
		}
		return out
	}())
}
func (h *Handler) Formats(c *gin.Context) {
	now := time.Now().UTC().Format(time.RFC3339)
	entries := []Entry{}
	for _, v := range []string{"epub", "pdf", "mobi", "azw3", "txt"} {
		p := "/opds/formats/" + v
		entries = append(entries, Entry{ID: h.URL(c, p), Title: strings.ToUpper(v), Updated: now, Links: []Link{{Rel: "subsection", Href: h.URL(c, p), Type: acqType}}})
	}
	h.navigationRows(c, "格式", entries)
}
func (h *Handler) navigationRows(c *gin.Context, title string, entries []Entry) {
	page, size := paging(c)
	total := len(entries)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	entries = entries[start:end]
	now := time.Now().UTC().Format(time.RFC3339)
	self := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		self += "?" + c.Request.URL.RawQuery
	}
	f := newFeed(h.URL(c, self), title, now)
	f.Links = h.commonLinks(c, self, navType)
	if page > 1 {
		f.Links = append(f.Links, Link{Rel: "previous", Href: h.pageURL(c, page-1, size), Type: navType})
	}
	if page*size < total {
		f.Links = append(f.Links, Link{Rel: "next", Href: h.pageURL(c, page+1, size), Type: navType})
	}
	f.Entries = entries
	h.xml(c, navType, f)
}
func (h *Handler) Cover(c *gin.Context) {
	mime, f, err := h.books.Cover(c.Param("id"))
	if err != nil {
		c.Status(404)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	c.DataFromReader(200, st.Size(), mime, f, nil)
}
func (h *Handler) Download(c *gin.Context) {
	bf, f, err := h.books.File(c.Param("id"), c.Param("fileId"))
	if err != nil {
		c.Status(404)
		return
	}
	defer f.Close()
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.QueryEscape(bf.OriginalName)))
	c.Header("Accept-Ranges", "bytes")
	c.DataFromReader(200, bf.FileSize, bf.MIMEType, f, nil)
}
func (h *Handler) OpenSearch(c *gin.Context) {
	doc := OpenSearch{XMLNS: "http://a9.com/-/spec/opensearch/1.1/", ShortName: "Bookshelf", Description: "Search the Bookshelf catalog", InputEncoding: "UTF-8", URL: OpenSearchURL{Type: acqType, Template: h.URL(c, "/opds/search?q={searchTerms}")}}
	h.xml(c, "application/opensearchdescription+xml", doc)
}
func (h *Handler) commonLinks(c *gin.Context, self, kind string) []Link {
	return []Link{{Rel: "self", Href: h.URL(c, self), Type: kind}, {Rel: "search", Href: h.URL(c, "/opensearch.xml"), Type: "application/opensearchdescription+xml"}}
}
func (h *Handler) pageURL(c *gin.Context, page, size int) string {
	q := c.Request.URL.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(size))
	return h.URL(c, c.Request.URL.Path+"?"+q.Encode())
}
func paging(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	s, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if p < 1 {
		p = 1
	}
	if s < 1 || s > 100 {
		s = 20
	}
	return p, s
}
func (h *Handler) xml(c *gin.Context, contentType string, v any) {
	data, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		c.Status(500)
		return
	}
	c.Data(200, contentType+"; charset=utf-8", append([]byte(xml.Header), data...))
}

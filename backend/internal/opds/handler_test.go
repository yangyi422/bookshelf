package opds

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bookshelf/internal/book"
	"bookshelf/internal/database"
	"bookshelf/internal/storage"
	"github.com/gin-gonic/gin"
)

func setupHandler(t *testing.T) (*gin.Engine, book.Detail) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(store.Root(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	svc := book.NewService(db, store, 1<<20)
	d, err := svc.Create(book.Input{Title: "A & B", Description: "<safe>", Language: "zh", ReadingStatus: "unread"})
	if err != nil {
		t.Fatal(err)
	}
	f := database.BookFile{ID: "file-1", BookID: d.ID, Format: "epub", MIMEType: "application/epub+zip", RelativePath: "books/missing.epub", OriginalName: "A & B.epub", FileSize: 10, SHA256: "unique"}
	if err = db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}
	d, _ = svc.Get(d.ID, false)
	h := New(db, svc, "https://books.example.com", "reader", "strong-reader-password")
	r := gin.New()
	g := r.Group("")
	g.Use(h.BasicAuth())
	g.GET("/opds", h.Root)
	g.GET("/opds/all", h.All)
	g.GET("/opds/search", h.Search)
	g.GET("/opensearch.xml", h.OpenSearch)
	return r, d
}
func request(r http.Handler, path string, auth, https bool) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if auth {
		req.SetBasicAuth("reader", "strong-reader-password")
	}
	if https {
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	r.ServeHTTP(w, req)
	return w
}
func TestBasicAuthAndHTTPS(t *testing.T) {
	r, _ := setupHandler(t)
	if w := request(r, "/opds", false, true); w.Code != 401 || w.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("unauthorized response: %d", w.Code)
	}
	if w := request(r, "/opds", true, false); w.Code != http.StatusUpgradeRequired {
		t.Fatalf("HTTP Basic allowed without HTTPS: %d", w.Code)
	}
	if w := request(r, "/opds", true, true); w.Code != 200 {
		t.Fatalf("authorized response: %d %s", w.Code, w.Body.String())
	}
}
func TestNavigationXML(t *testing.T) {
	r, _ := setupHandler(t)
	w := request(r, "/opds", true, true)
	if !strings.Contains(w.Header().Get("Content-Type"), "kind=navigation") {
		t.Fatalf("content type %q", w.Header().Get("Content-Type"))
	}
	var feed Feed
	if err := xml.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Entries) != 5 || feed.Title != "Bookshelf" {
		t.Fatalf("unexpected feed: %#v", feed)
	}
}
func TestAcquisitionLinksAndEscaping(t *testing.T) {
	r, d := setupHandler(t)
	w := request(r, "/opds/all", true, true)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "<title>A & B</title>") {
		t.Fatal("XML title was not escaped")
	}
	if !strings.Contains(body, "A &amp; B") || !strings.Contains(body, "http://opds-spec.org/acquisition") || !strings.Contains(body, "/opds/books/"+d.ID+"/files/file-1") {
		t.Fatalf("missing acquisition data: %s", body)
	}
	var feed Feed
	if err := xml.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Entries) != 1 || len(feed.Entries[0].Links) < 2 {
		t.Fatalf("unexpected entries: %#v", feed.Entries)
	}
}
func TestOpenSearch(t *testing.T) {
	r, _ := setupHandler(t)
	w := request(r, "/opensearch.xml", true, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "{searchTerms}") {
		t.Fatalf("invalid OpenSearch: %d %s", w.Code, w.Body.String())
	}
}

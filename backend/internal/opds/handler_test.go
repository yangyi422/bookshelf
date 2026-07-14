package opds

import (
	"crypto/tls"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bookshelf/internal/auth"
	"bookshelf/internal/book"
	"bookshelf/internal/database"
	"bookshelf/internal/proxy"
	appsettings "bookshelf/internal/settings"
	"bookshelf/internal/storage"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type fixture struct {
	r        *gin.Engine
	detail   book.Detail
	db       *gorm.DB
	settings *appsettings.Service
}

func setupHandler(t *testing.T) fixture {
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
	hash, _ := auth.HashPassword("strong-reader-password")
	row := database.SystemSetting{ID: 1, OPDSEnabled: true, OPDSAccessMode: appsettings.ModeHTTPSOnly, OPDSUsername: "reader", OPDSPasswordHash: hash, PublicBaseURL: "https://books.example.com"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	settingsService := appsettings.New(db, appsettings.Defaults{})
	resolver, _ := proxy.New([]string{"10.0.0.0/8"})
	h := New(db, svc, settingsService, resolver)
	r := gin.New()
	g := r.Group("")
	g.Use(h.BasicAuth())
	g.GET("/opds", h.Root)
	g.GET("/opds/all", h.All)
	g.GET("/opds/search", h.Search)
	g.GET("/opensearch.xml", h.OpenSearch)
	g.GET("/opds/books/:id/cover", h.Cover)
	g.GET("/opds/books/:id/files/:fileId", h.Download)
	return fixture{r: r, detail: d, db: db, settings: settingsService}
}

func request(r http.Handler, path, username, password string, tlsRequest, trustedProxy bool) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	if tlsRequest {
		req.TLS = &tls.ConnectionState{}
	}
	if trustedProxy {
		req.RemoteAddr = "10.1.2.3:4567"
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	r.ServeHTTP(w, req)
	return w
}

func authRequest(f fixture, path string, tlsRequest bool) *httptest.ResponseRecorder {
	return request(f.r, path, "reader", "strong-reader-password", tlsRequest, false)
}

func TestOPDSAccessModesAndCredentials(t *testing.T) {
	f := setupHandler(t)
	f.db.Model(&database.SystemSetting{}).Where("id=1").Updates(map[string]any{"opds_enabled": false, "opds_access_mode": appsettings.ModeDisabled})
	if w := authRequest(f, "/opds", true); w.Code != 404 {
		t.Fatalf("disabled = %d", w.Code)
	}
	f.db.Model(&database.SystemSetting{}).Where("id=1").Updates(map[string]any{"opds_enabled": true, "opds_access_mode": appsettings.ModeHTTPSOnly})
	if w := authRequest(f, "/opds", false); w.Code != 403 {
		t.Fatalf("https_only HTTP = %d", w.Code)
	}
	if w := authRequest(f, "/opds", true); w.Code != 200 {
		t.Fatalf("TLS = %d", w.Code)
	}
	if w := request(f.r, "/opds", "reader", "strong-reader-password", false, true); w.Code != 200 {
		t.Fatalf("trusted proxy = %d", w.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/opds", nil)
	req.SetBasicAuth("reader", "strong-reader-password")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.RemoteAddr = "203.0.113.5:1234"
	w := httptest.NewRecorder()
	f.r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("forged proxy = %d", w.Code)
	}
	f.db.Model(&database.SystemSetting{}).Where("id=1").Update("opds_access_mode", appsettings.ModeHTTPAndHTTPS)
	if w := authRequest(f, "/opds", false); w.Code != 200 {
		t.Fatalf("mixed HTTP = %d", w.Code)
	}
	if w := authRequest(f, "/opds", true); w.Code != 200 {
		t.Fatalf("mixed HTTPS = %d", w.Code)
	}
	if w := request(f.r, "/opds", "reader", "wrong-password", true, false); w.Code != 401 {
		t.Fatalf("wrong credentials = %d", w.Code)
	}
	if w := request(f.r, "/opds", "", "", true, false); w.Code != 401 {
		t.Fatalf("missing credentials = %d", w.Code)
	}
}

func TestProtectedFilesCannotBypassAuth(t *testing.T) {
	f := setupHandler(t)
	for _, path := range []string{"/opds/books/" + f.detail.ID + "/cover", "/opds/books/" + f.detail.ID + "/files/file-1"} {
		if w := request(f.r, path, "", "", true, false); w.Code != 401 {
			t.Fatalf("%s = %d", path, w.Code)
		}
	}
}

func TestSettingsChangesApplyWithoutRestart(t *testing.T) {
	f := setupHandler(t)
	if w := authRequest(f, "/opds", false); w.Code != 403 {
		t.Fatal(w.Code)
	}
	f.db.Model(&database.SystemSetting{}).Where("id=1").Update("opds_access_mode", appsettings.ModeHTTPAndHTTPS)
	if w := authRequest(f, "/opds", false); w.Code != 200 {
		t.Fatal(w.Code)
	}
	f.db.Model(&database.SystemSetting{}).Where("id=1").Update("public_base_url", "https://new.example:8443")
	w := authRequest(f, "/opds", false)
	if !strings.Contains(w.Body.String(), "https://new.example:8443/opds") || strings.Contains(w.Body.String(), "new.example:8443//opds") {
		t.Fatalf("updated links: %s", w.Body.String())
	}
}

func TestNavigationXML(t *testing.T) {
	f := setupHandler(t)
	w := authRequest(f, "/opds", true)
	var feed Feed
	if err := xml.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Entries) != 5 || feed.Title != "Bookshelf" {
		t.Fatalf("unexpected feed: %#v", feed)
	}
}
func TestAcquisitionLinksAndEscaping(t *testing.T) {
	f := setupHandler(t)
	w := authRequest(f, "/opds/all", true)
	body := w.Body.String()
	if w.Code != 200 || strings.Contains(body, "<title>A & B</title>") || !strings.Contains(body, "A &amp; B") || !strings.Contains(body, "http://opds-spec.org/acquisition") || !strings.Contains(body, "/opds/books/"+f.detail.ID+"/files/file-1") {
		t.Fatalf("invalid acquisition: %d %s", w.Code, body)
	}
}
func TestOpenSearch(t *testing.T) {
	f := setupHandler(t)
	w := authRequest(f, "/opensearch.xml", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "{searchTerms}") {
		t.Fatalf("invalid OpenSearch: %d %s", w.Code, w.Body.String())
	}
}

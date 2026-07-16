package auth

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bookshelf/internal/database"
	"bookshelf/internal/proxy"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSessionCookieSecureModes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		mode       string
		tls        bool
		remoteAddr string
		forwarded  string
		secure     bool
	}{
		{name: "auto direct HTTP", mode: "auto", remoteAddr: "203.0.113.10:1234", secure: false},
		{name: "auto direct HTTPS", mode: "auto", tls: true, remoteAddr: "203.0.113.10:1234", secure: true},
		{name: "auto trusted proxy HTTPS", mode: "auto", remoteAddr: "10.2.3.4:1234", forwarded: "https", secure: true},
		{name: "auto forged proxy HTTPS", mode: "auto", remoteAddr: "203.0.113.10:1234", forwarded: "https", secure: false},
		{name: "always over HTTP", mode: "always", remoteAddr: "203.0.113.10:1234", secure: true},
		{name: "never over HTTPS", mode: "never", tls: true, remoteAddr: "203.0.113.10:1234", secure: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := testHandler(t, tt.mode)
			r := gin.New()
			r.POST("/login", h.Login)
			req := httptest.NewRequest(http.MethodPost, "http://books.example/login", bytes.NewBufferString(`{"username":"admin","password":"correct horse battery staple"}`))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = tt.remoteAddr
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwarded)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("login status = %d, body = %s", w.Code, w.Body.String())
			}
			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("cookies = %d", len(cookies))
			}
			cookie := cookies[0]
			if cookie.Secure != tt.secure {
				t.Fatalf("Secure = %t, want %t", cookie.Secure, tt.secure)
			}
			if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Domain != "" {
				t.Fatalf("unexpected cookie attributes: %#v", cookie)
			}
		})
	}
}

func TestLogoutDeletesCookieUsingRequestSecurity(t *testing.T) {
	h := testHandler(t, "auto")
	r := gin.New()
	r.POST("/login", h.Login)
	r.POST("/logout", h.Logout)
	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{"username":"admin","password":"correct horse battery staple"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(login, loginRequest)
	cookie := login.Result().Cookies()[0]

	logout := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutRequest.AddCookie(cookie)
	r.ServeHTTP(logout, logoutRequest)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logout.Code)
	}
	deleted := logout.Result().Cookies()
	if len(deleted) != 1 || deleted[0].Value != "" || deleted[0].MaxAge >= 0 || deleted[0].Secure {
		t.Fatalf("invalid deletion cookie: %#v", deleted)
	}
}

func testHandler(t *testing.T, mode string) *Handler {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "-") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.User{}, &database.Session{}); err != nil {
		t.Fatal(err)
	}
	service := New(db, time.Hour)
	if err := service.BootstrapAdmin("admin", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	resolver, err := proxy.New([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(service, mode, resolver, time.Hour)
}

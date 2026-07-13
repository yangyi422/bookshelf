package server

import (
	"bookshelf/internal/auth"
	"bookshelf/internal/config"
	"bookshelf/internal/database"
	"bookshelf/internal/storage"
	"bytes"
	"encoding/json"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginAndMe(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	_ = db.AutoMigrate(&database.User{}, &database.Session{})
	svc := auth.New(db, time.Hour)
	if err := svc.BootstrapAdmin("admin", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Environment: "development", SessionTTL: time.Hour}
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := New(cfg, db, store, svc)
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "correct horse battery staple"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("login status %d: %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("session cookie flags missing")
	}
	me := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(cookies[0])
	r.ServeHTTP(me, req)
	if me.Code != 200 {
		t.Fatalf("me status %d", me.Code)
	}
}

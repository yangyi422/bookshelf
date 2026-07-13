package auth

import (
	"bookshelf/internal/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func testService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.User{}, &database.Session{}); err != nil {
		t.Fatal(err)
	}
	return New(db, time.Hour)
}
func TestPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") || VerifyPassword(hash, "wrong password") {
		t.Fatal("password verification mismatch")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected short password rejection")
	}
}
func TestBootstrapLoginAndSession(t *testing.T) {
	s := testService(t)
	if err := s.BootstrapAdmin("admin", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	u, token, err := s.Login("admin", "correct horse battery staple")
	if err != nil || token == "" || u.Role != "admin" {
		t.Fatalf("login failed: %v", err)
	}
	if _, err := s.UserForToken(token); err != nil {
		t.Fatal(err)
	}
	if err := s.Logout(token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserForToken(token); err == nil {
		t.Fatal("logged-out token accepted")
	}
}

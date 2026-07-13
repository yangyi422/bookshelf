package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"bookshelf/internal/database"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var ErrInvalidCredentials = errors.New("invalid username or password")

type Service struct {
	db  *gorm.DB
	ttl time.Duration
}

func New(db *gorm.DB, ttl time.Duration) *Service { return &Service{db: db, ttl: ttl} }
func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", errors.New("password must contain at least 12 characters")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (s *Service) BootstrapAdmin(username, password string) error {
	var count int64
	if err := s.db.Model(&database.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if username == "" || password == "" {
		return errors.New("no users exist: ADMIN_USERNAME and ADMIN_PASSWORD are required")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash bootstrap administrator password: %w", err)
	}
	u := database.User{ID: uuid.NewString(), Username: username, PasswordHash: hash, DisplayName: username, Role: "admin", Enabled: true}
	if err := s.db.Create(&u).Error; err != nil {
		return fmt.Errorf("create bootstrap administrator: %w", err)
	}
	return nil
}

func (s *Service) Login(username, password string) (database.User, string, error) {
	var u database.User
	if err := s.db.Where("username = ? AND enabled = ?", username, true).First(&u).Error; err != nil {
		return u, "", ErrInvalidCredentials
	}
	if !VerifyPassword(u.PasswordHash, password) {
		return u, "", ErrInvalidCredentials
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return u, "", fmt.Errorf("generate session: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	sess := database.Session{IDHash: tokenHash(token), UserID: u.ID, ExpiresAt: time.Now().Add(s.ttl)}
	if err := s.db.Create(&sess).Error; err != nil {
		return u, "", fmt.Errorf("store session: %w", err)
	}
	return u, token, nil
}
func (s *Service) UserForToken(token string) (database.User, error) {
	var u database.User
	if token == "" {
		return u, ErrInvalidCredentials
	}
	var sess database.Session
	if err := s.db.Where("id_hash = ? AND expires_at > ?", tokenHash(token), time.Now()).First(&sess).Error; err != nil {
		return u, ErrInvalidCredentials
	}
	if err := s.db.Where("id = ? AND enabled = ?", sess.UserID, true).First(&u).Error; err != nil {
		return u, ErrInvalidCredentials
	}
	return u, nil
}
func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	return s.db.Delete(&database.Session{}, "id_hash = ?", tokenHash(token)).Error
}
func (s *Service) ChangePassword(userID, current, replacement string) error {
	var u database.User
	if err := s.db.First(&u, "id = ?", userID).Error; err != nil {
		return ErrInvalidCredentials
	}
	if !VerifyPassword(u.PasswordHash, current) {
		return ErrInvalidCredentials
	}
	hash, err := HashPassword(replacement)
	if err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&u).Update("password_hash", hash).Error; err != nil {
			return err
		}
		return tx.Delete(&database.Session{}, "user_id = ?", userID).Error
	})
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

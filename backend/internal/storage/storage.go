package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type Storage struct{ root string }

func New(root string) (*Storage, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	for _, d := range []string{"books", "imports", "cache", "trash", "backups", "manifests"} {
		if err := os.MkdirAll(filepath.Join(abs, d), 0750); err != nil {
			return nil, fmt.Errorf("create data directory %s: %w", d, err)
		}
	}
	probe, err := os.CreateTemp(abs, ".write-check-")
	if err != nil {
		return nil, fmt.Errorf("data directory is not writable: %w", err)
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		return nil, err
	}
	if err := os.Remove(name); err != nil {
		return nil, err
	}
	return &Storage{root: abs}, nil
}

func (s *Storage) Root() string                    { return s.root }
func (s *Storage) Relative(parts ...string) string { return filepath.ToSlash(filepath.Join(parts...)) }
func (s *Storage) Resolve(relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", errors.New("path must be a non-empty relative path")
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes data directory")
	}
	full := filepath.Join(s.root, clean)
	rel, err := filepath.Rel(s.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes data directory")
	}
	return full, nil
}

func (s *Storage) EnsureDir(relative string) (string, error) {
	full, err := s.Resolve(relative)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(full, 0750); err != nil {
		return "", err
	}
	return full, nil
}
func (s *Storage) Move(fromRelative, toRelative string) error {
	from, err := s.Resolve(fromRelative)
	if err != nil {
		return err
	}
	to, err := s.Resolve(toRelative)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0750); err != nil {
		return err
	}
	if _, err := os.Stat(to); err == nil {
		return errors.New("destination already exists")
	}
	return os.Rename(from, to)
}
func (s *Storage) Remove(relative string) error {
	full, err := s.Resolve(relative)
	if err != nil {
		return err
	}
	return os.RemoveAll(full)
}
func (s *Storage) Open(relative string) (*os.File, error) {
	full, err := s.Resolve(relative)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func SHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

var formats = map[string]string{".epub": "application/epub+zip", ".pdf": "application/pdf", ".mobi": "application/x-mobipocket-ebook", ".azw3": "application/vnd.amazon.ebook", ".txt": "text/plain; charset=utf-8"}

func FormatForName(name string) (format, mime string, err error) {
	ext := strings.ToLower(filepath.Ext(name))
	mime, ok := formats[ext]
	if !ok {
		return "", "", fmt.Errorf("unsupported file extension %q", ext)
	}
	return strings.TrimPrefix(ext, "."), mime, nil
}

// ValidateHeader checks server-observed bytes; a client-provided Content-Type is never trusted.
func ValidateHeader(format string, header []byte) error {
	valid := false
	switch format {
	case "epub":
		valid = len(header) >= 4 && string(header[:4]) == "PK\x03\x04"
	case "pdf":
		valid = len(header) >= 5 && string(header[:5]) == "%PDF-"
	case "mobi", "azw3":
		valid = len(header) >= 68 && string(header[60:68]) == "BOOKMOBI"
	case "txt":
		valid = utf8.Valid(header) && !strings.ContainsRune(string(header), '\x00')
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	if !valid {
		return fmt.Errorf("file content does not match %s format", format)
	}
	return nil
}

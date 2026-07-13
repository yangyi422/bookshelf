package scanner

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bookshelf/internal/database"
	"bookshelf/internal/storage"
	"gorm.io/gorm"
)

type Issue struct {
	Code    string `json:"code"`
	BookID  string `json:"book_id,omitempty"`
	FileID  string `json:"file_id,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}
type Report struct {
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
	Issues       []Issue   `json:"issues"`
	CheckedBooks int       `json:"checked_books"`
	CheckedFiles int       `json:"checked_files"`
	TrashEntries int       `json:"trash_entries"`
	Status       string    `json:"status"`
}
type Manifest struct {
	Version     int                   `json:"version"`
	GeneratedAt time.Time             `json:"generated_at"`
	Books       []database.Book       `json:"books"`
	Files       []database.BookFile   `json:"files"`
	Authors     []database.Author     `json:"authors"`
	Tags        []database.Tag        `json:"tags"`
	BookAuthors []database.BookAuthor `json:"book_authors"`
	BookTags    []database.BookTag    `json:"book_tags"`
}
type Service struct {
	db    *gorm.DB
	store *storage.Storage
	mu    sync.RWMutex
	last  Report
}

func New(db *gorm.DB, store *storage.Storage) *Service {
	return &Service{db: db, store: store, last: Report{Status: "idle"}}
}
func (s *Service) Scan() (Report, error) {
	r := Report{StartedAt: time.Now().UTC(), Status: "running"}
	s.set(r)
	var books []database.Book
	if err := s.db.Where("deleted_at IS NULL").Find(&books).Error; err != nil {
		return r, err
	}
	r.CheckedBooks = len(books)
	bookIDs := map[string]bool{}
	known := map[string]bool{}
	for _, b := range books {
		bookIDs[b.ID] = true
		meta := s.store.Relative("books", b.ID, "metadata.json")
		known[meta] = true
		if !exists(s.store, meta) {
			r.Issues = append(r.Issues, Issue{Code: "metadata_missing", BookID: b.ID, Path: meta, Message: "metadata.json is missing"})
		}
		if b.CoverPath != "" {
			known[b.CoverPath] = true
			if !exists(s.store, b.CoverPath) {
				r.Issues = append(r.Issues, Issue{Code: "cover_missing", BookID: b.ID, Path: b.CoverPath, Message: "cover file is missing"})
			}
		}
	}
	var files []database.BookFile
	if err := s.db.Raw("SELECT book_files.* FROM book_files JOIN books ON books.id=book_files.book_id WHERE books.deleted_at IS NULL").Scan(&files).Error; err != nil {
		return r, err
	}
	r.CheckedFiles = len(files)
	allowed := map[string]bool{"epub": true, "pdf": true, "mobi": true, "azw3": true, "txt": true}
	for _, f := range files {
		known[f.RelativePath] = true
		if !allowed[f.Format] {
			r.Issues = append(r.Issues, Issue{Code: "invalid_format", BookID: f.BookID, FileID: f.ID, Path: f.RelativePath, Message: "database format is not supported"})
			continue
		}
		handle, err := s.store.Open(f.RelativePath)
		if err != nil {
			r.Issues = append(r.Issues, Issue{Code: "file_missing", BookID: f.BookID, FileID: f.ID, Path: f.RelativePath, Message: "book file is missing"})
			continue
		}
		sum, e := storage.SHA256(handle)
		_ = handle.Close()
		if e != nil {
			return r, e
		}
		if !strings.EqualFold(sum, f.SHA256) {
			r.Issues = append(r.Issues, Issue{Code: "checksum_mismatch", BookID: f.BookID, FileID: f.ID, Path: f.RelativePath, Message: "SHA-256 does not match database"})
		}
	}
	booksRoot, _ := s.store.Resolve("books")
	_ = filepath.WalkDir(booksRoot, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		rel, _ := filepath.Rel(s.store.Root(), p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if filepath.Dir(rel) == "books" && rel != "books" && !bookIDs[filepath.Base(rel)] {
				r.Issues = append(r.Issues, Issue{Code: "orphan_directory", Path: rel, Message: "book directory has no active database record"})
			}
			return nil
		}
		if !known[rel] {
			r.Issues = append(r.Issues, Issue{Code: "untracked_file", Path: rel, Message: "file has no database record"})
		}
		return nil
	})
	trashRoot, _ := s.store.Resolve("trash")
	_ = filepath.WalkDir(trashRoot, func(p string, d fs.DirEntry, e error) error {
		if e == nil && p != trashRoot && d.IsDir() {
			r.TrashEntries++
		}
		return nil
	})
	r.FinishedAt = time.Now().UTC()
	r.Status = "completed"
	s.set(r)
	return r, nil
}
func exists(store *storage.Storage, rel string) bool {
	p, e := store.Resolve(rel)
	if e != nil {
		return false
	}
	_, e = os.Stat(p)
	return e == nil
}
func (s *Service) Last() Report { s.mu.RLock(); defer s.mu.RUnlock(); return s.last }
func (s *Service) set(r Report) { s.mu.Lock(); s.last = r; s.mu.Unlock() }
func (s *Service) ExportManifest() (Manifest, error) {
	m := Manifest{Version: 1, GeneratedAt: time.Now().UTC()}
	if err := s.db.Find(&m.Books).Error; err != nil {
		return m, err
	}
	for _, target := range []any{&m.Files, &m.Authors, &m.Tags, &m.BookAuthors, &m.BookTags} {
		if err := s.db.Find(target).Error; err != nil {
			return m, err
		}
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return m, err
	}
	path, err := s.store.Resolve(s.store.Relative("manifests", "manifest.json"))
	if err != nil {
		return m, err
	}
	if err = os.WriteFile(path, raw, 0640); err != nil {
		return m, err
	}
	return m, nil
}

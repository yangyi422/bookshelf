package book

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"

	"bookshelf/internal/database"
	"bookshelf/internal/storage"
)

func setup(t *testing.T) (*Service, *storage.Storage) {
	t.Helper()
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(store.Root(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(db, store, 1024*1024), store
}
func fileHeader(t *testing.T, name string, data []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = w.Close()
	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(2 << 20); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["file"][0]
}
func TestCreateUploadDeleteRestore(t *testing.T) {
	s, store := setup(t)
	d, err := s.Create(Input{Title: "Test", ReadingStatus: "unread"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.AddFile(d.ID, fileHeader(t, "sample.pdf", []byte("%PDF-1.7\ncontent")))
	if err != nil {
		t.Fatal(err)
	}
	if f.RelativePath == "" || f.SHA256 == "" {
		t.Fatal("file metadata missing")
	}
	path, _ := store.Resolve(f.RelativePath)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFile(d.ID, fileHeader(t, "duplicate.pdf", []byte("%PDF-1.7\ncontent"))); err != ErrDuplicate {
		t.Fatalf("duplicate error = %v", err)
	}
	if err := s.Delete(d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(d.ID, false); err != ErrNotFound {
		t.Fatalf("deleted book visible: %v", err)
	}
	trash, err := s.Trash()
	if err != nil || len(trash) != 1 || trash[0].ID != d.ID {
		t.Fatalf("trash listing failed: %#v %v", trash, err)
	}
	if err := s.Restore(d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(d.ID, false); err != nil {
		t.Fatal(err)
	}
}
func TestRejectsMismatchedContent(t *testing.T) {
	s, _ := setup(t)
	d, _ := s.Create(Input{Title: "Bad", ReadingStatus: "unread"})
	if _, err := s.AddFile(d.ID, fileHeader(t, "fake.pdf", []byte("not pdf"))); err == nil {
		t.Fatal("mismatched content accepted")
	}
}
func TestImportAppliesPDFMetadata(t *testing.T) {
	s, _ := setup(t)
	d, err := s.Import(fileHeader(t, "fallback.pdf", []byte("%PDF-1.7\n/Title (Parsed Title) /Author (Writer) /Type /Page >>")))
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Parsed Title" || len(d.Authors) != 1 || d.Authors[0].Name != "Writer" || len(d.Files) != 1 || d.Files[0].PageCount != 1 {
		t.Fatalf("metadata not applied: %#v", d)
	}
}
func TestListFiltersAndSortWhitelist(t *testing.T) {
	s, _ := setup(t)
	a := database.Author{ID: "author-1", Name: "Alice", SortName: "Alice"}
	tag := database.Tag{ID: "tag-1", Name: "SciFi"}
	if err := s.db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}
	d, err := s.Create(Input{Title: "Filtered", ReadingStatus: "reading", AuthorIDs: []string{a.ID}, TagIDs: []string{tag.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddFile(d.ID, fileHeader(t, "book.pdf", []byte("%PDF-1.7"))); err != nil {
		t.Fatal(err)
	}
	items, total, err := s.List(ListOptions{AuthorID: a.ID, TagID: tag.ID, Format: "pdf", ReadingStatus: "reading", Sort: "title", Order: "asc", Page: 1, Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != d.ID {
		t.Fatalf("unexpected filtered result: %d %#v", total, items)
	}
}

package backup

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"testing"

	"bookshelf/internal/database"
	"bookshelf/internal/storage"
)

func TestCreateAndValidateBackup(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(store.Root(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&database.Book{ID: "book-1", Title: "Backup Book", ReadingStatus: "unread"}).Error; err != nil {
		t.Fatal(err)
	}
	bookDir, _ := store.EnsureDir("books/book-1")
	if err = os.WriteFile(bookDir+"/metadata.json", []byte(`{"title":"Backup Book"}`), 0600); err != nil {
		t.Fatal(err)
	}
	svc := New(db, store, 30)
	record, err := svc.Create()
	if err != nil {
		t.Fatal(err)
	}
	if record.Checksum == "" || record.FileSize == 0 {
		t.Fatalf("invalid backup record: %#v", record)
	}
	if _, err = svc.Validate(record.ID); err != nil {
		t.Fatal(err)
	}
	if sidecar, err := store.Resolve(record.FilePath + ".sha256"); err != nil {
		t.Fatal(err)
	} else if _, err = os.Stat(sidecar); err != nil {
		t.Fatal("checksum sidecar missing")
	}
	f, err := store.Open(record.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	names := map[string]bool{}
	for {
		hdr, e := tr.Next()
		if e != nil {
			break
		}
		names[hdr.Name] = true
	}
	_ = gz.Close()
	_ = f.Close()
	for _, name := range []string{"library.db", "books/book-1/metadata.json", "backup.json"} {
		if !names[name] {
			t.Errorf("archive missing %s", name)
		}
	}
}

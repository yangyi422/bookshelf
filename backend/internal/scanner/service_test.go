package scanner

import (
	"os"
	"testing"

	"bookshelf/internal/book"
	"bookshelf/internal/database"
	"bookshelf/internal/storage"
)

func TestScanFindsMissingAndOrphanData(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(store.Root(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	books := book.NewService(db, store, 1<<20)
	d, err := books.Create(book.Input{Title: "Scan", ReadingStatus: "unread"})
	if err != nil {
		t.Fatal(err)
	}
	missing := database.BookFile{ID: "missing", BookID: d.ID, Format: "pdf", MIMEType: "application/pdf", RelativePath: "books/" + d.ID + "/files/missing.pdf", OriginalName: "missing.pdf", SHA256: "abc"}
	if err = db.Create(&missing).Error; err != nil {
		t.Fatal(err)
	}
	orphan, _ := store.EnsureDir("books/orphan-book")
	if err = os.WriteFile(orphan+"/unknown.txt", []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := New(db, store)
	report, err := svc.Scan()
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, issue := range report.Issues {
		codes[issue.Code] = true
	}
	if !codes["file_missing"] || !codes["orphan_directory"] || !codes["untracked_file"] {
		t.Fatalf("missing expected issues: %#v", report.Issues)
	}
	manifest, err := svc.ExportManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Books) != 1 || !exists(store, "manifests/manifest.json") {
		t.Fatalf("invalid manifest: %#v", manifest)
	}
}

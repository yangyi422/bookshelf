package metadata

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func makeEPUB(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "test.epub")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for n, data := range entries {
		p, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = p.Write(data)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}
func TestParseEPUBMetadataAndCover(t *testing.T) {
	name := makeEPUB(t, map[string][]byte{"META-INF/container.xml": []byte(`<?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`), "OEBPS/content.opf": []byte(`<?xml version="1.0"?><package><metadata><title>Test Book</title><creator>Alice</creator><language>zh</language><publisher>Press</publisher><identifier>urn:isbn:978-1-234</identifier><description>Summary</description><meta name="cover" content="cover-id"/></metadata><manifest><item id="cover-id" href="cover.jpg" media-type="image/jpeg"/></manifest></package>`), "OEBPS/cover.jpg": []byte{0xff, 0xd8, 0xff, 0xd9}})
	r, err := Parse(name, "epub")
	if err != nil {
		t.Fatal(err)
	}
	if r.Title != "Test Book" || len(r.Authors) != 1 || r.Authors[0] != "Alice" || r.CoverExt != ".jpg" {
		t.Fatalf("unexpected metadata: %#v", r)
	}
}
func TestEPUBRejectsTraversal(t *testing.T) {
	name := makeEPUB(t, map[string][]byte{"../evil": []byte("x")})
	if _, err := Parse(name, "epub"); err == nil {
		t.Fatal("unsafe EPUB accepted")
	}
}
func TestParsePDF(t *testing.T) {
	name := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(name, []byte("%PDF-1.7\n/Title (PDF Title) /Author (Bob) /Type /Page >> /Type /Page >>"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Parse(name, "pdf")
	if err != nil {
		t.Fatal(err)
	}
	if r.Title != "PDF Title" || len(r.Authors) != 1 || r.PageCount != 2 {
		t.Fatalf("unexpected PDF metadata: %#v", r)
	}
}

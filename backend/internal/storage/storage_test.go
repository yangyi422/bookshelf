package storage

import (
	"strings"
	"testing"
)

func TestResolveRejectsTraversal(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"../secret", "/etc/passwd", "books/../../secret", ""} {
		if _, err := s.Resolve(p); err == nil {
			t.Errorf("Resolve(%q) should fail", p)
		}
	}
}
func TestFormatsAndChecksum(t *testing.T) {
	format, mime, err := FormatForName("Book.EPUB")
	if err != nil || format != "epub" || mime != "application/epub+zip" {
		t.Fatalf("unexpected format result: %q %q %v", format, mime, err)
	}
	if _, _, err := FormatForName("bad.exe"); err == nil {
		t.Fatal("expected unsupported format")
	}
	sum, err := SHA256(strings.NewReader("abc"))
	if err != nil || sum != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("unexpected checksum %q: %v", sum, err)
	}
}

func TestValidateHeader(t *testing.T) {
	mobi := make([]byte, 68)
	copy(mobi[60:], "BOOKMOBI")
	for format, header := range map[string][]byte{"epub": []byte("PK\x03\x04rest"), "pdf": []byte("%PDF-1.7"), "mobi": mobi, "azw3": mobi, "txt": []byte("valid text")} {
		if err := ValidateHeader(format, header); err != nil {
			t.Errorf("%s rejected: %v", format, err)
		}
	}
	if err := ValidateHeader("pdf", []byte("not a pdf")); err == nil {
		t.Fatal("invalid PDF accepted")
	}
}

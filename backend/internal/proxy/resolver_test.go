package proxy

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

func TestHTTPSAndOriginTrustBoundaries(t *testing.T) {
	r, _ := New([]string{"10.0.0.0/8"})
	direct := httptest.NewRequest("GET", "http://internal/opds", nil)
	direct.Host = "books.example:8080"
	direct.TLS = &tls.ConnectionState{}
	if !r.HTTPS(direct) || r.Origin(direct, "") != "https://books.example:8080" {
		t.Fatal("direct TLS not detected")
	}
	trusted := httptest.NewRequest("GET", "http://internal/opds", nil)
	trusted.RemoteAddr = "10.2.3.4:1234"
	trusted.Header.Set("X-Forwarded-Proto", "https")
	trusted.Header.Set("X-Forwarded-Host", "catalog.example:8443")
	if !r.HTTPS(trusted) || r.Origin(trusted, "") != "https://catalog.example:8443" {
		t.Fatal("trusted proxy headers not applied")
	}
	forged := httptest.NewRequest("GET", "http://internal/opds", nil)
	forged.RemoteAddr = "203.0.113.2:1234"
	forged.Header.Set("X-Forwarded-Proto", "https")
	forged.Header.Set("X-Forwarded-Host", "evil.example")
	if r.HTTPS(forged) || r.Origin(forged, "") == "https://evil.example" {
		t.Fatal("untrusted proxy headers applied")
	}
}

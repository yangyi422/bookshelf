package proxy

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Resolver struct{ networks []*net.IPNet }

func New(cidrs []string) (*Resolver, error) {
	r := &Resolver{}
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if ip := net.ParseIP(raw); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			r.networks = append(r.networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, err
		}
		r.networks = append(r.networks, network)
	}
	return r, nil
}

func (r *Resolver) Trusted(req *http.Request) bool {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	ip := net.ParseIP(host)
	for _, network := range r.networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (r *Resolver) HTTPS(req *http.Request) bool {
	if req.TLS != nil {
		return true
	}
	return r.Trusted(req) && first(req.Header.Get("X-Forwarded-Proto")) == "https"
}

func (r *Resolver) Origin(req *http.Request, configured string) string {
	if configured = strings.TrimRight(strings.TrimSpace(configured), "/"); configured != "" {
		return configured
	}
	scheme := "http"
	host := req.Host
	if req.TLS != nil {
		scheme = "https"
	} else if r.Trusted(req) {
		if forwarded := first(req.Header.Get("X-Forwarded-Proto")); forwarded == "http" || forwarded == "https" {
			scheme = forwarded
		}
		if forwardedHost := first(req.Header.Get("X-Forwarded-Host")); validHost(forwardedHost) {
			host = forwardedHost
		}
	}
	return scheme + "://" + host
}

func first(v string) string { return strings.ToLower(strings.TrimSpace(strings.Split(v, ",")[0])) }

func validHost(host string) bool {
	if host == "" || strings.ContainsAny(host, "/\\\r\n\t ") {
		return false
	}
	u, err := url.Parse("http://" + host)
	return err == nil && u.Host == host
}

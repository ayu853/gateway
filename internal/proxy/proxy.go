package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ayu853/gateway/internal/loadbalancer"
)

// Proxy is the core reverse proxy engine that forwards requests to backends.
type Proxy struct {
	balancer  loadbalancer.Balancer
	transport http.RoundTripper
	proxyPool sync.Pool
}

// New creates a new Proxy with the given load balancer.
func New(balancer loadbalancer.Balancer) *Proxy {
	p := &Proxy{
		balancer: balancer,
		transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}
	return p
}

// ServeHTTP handles incoming requests by proxying them to a backend.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend := p.balancer.Next(r)
	if backend == nil {
		http.Error(w, `{"error": "no healthy backends available"}`, http.StatusServiceUnavailable)
		return
	}

	targetURL, err := url.Parse(backend.URL)
	if err != nil {
		http.Error(w, `{"error": "invalid backend URL"}`, http.StatusInternalServerError)
		return
	}

	// Track active connections for least-connections balancing
	backend.IncrementConnections()
	defer backend.DecrementConnections()

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Host = targetURL.Host

			// Set X-Forwarded headers
			if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
				if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
					clientIP = prior + ", " + clientIP
				}
				req.Header.Set("X-Forwarded-For", clientIP)
			}
			req.Header.Set("X-Forwarded-Host", req.Host)
			req.Header.Set("X-Forwarded-Proto", schemeFromRequest(req))
			req.Header.Set("X-Gateway-Backend", backend.URL)
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Set("X-Gateway", "hp-gateway/1.0")
			resp.Header.Set("X-Backend-Server", backend.URL)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			backend.MarkUnhealthy()
			http.Error(w, fmt.Sprintf(`{"error": "backend unavailable", "detail": "%s"}`, err.Error()), http.StatusBadGateway)
		},
		Transport: p.transport,
	}

	proxy.ServeHTTP(w, r)
}

func schemeFromRequest(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if scheme := r.Header.Get("X-Forwarded-Proto"); scheme != "" {
		return scheme
	}
	if strings.HasPrefix(r.RequestURI, "https") {
		return "https"
	}
	return "http"
}

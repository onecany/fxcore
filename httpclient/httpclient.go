// Package httpclient provides a proxy-aware HTTP client factory.
// When HTTP_PROXY, HTTPS_PROXY, or ALL_PROXY environment variables are set,
// all clients created via New() will route traffic through the specified proxy.
// Supports socks5://, socks://, http://, and https:// proxy schemes.
package httpclient

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// New creates an *http.Client with the given timeout and proxy support.
// Proxy is configured via environment variables: ALL_PROXY, HTTPS_PROXY, HTTP_PROXY.
// If no proxy is configured, the client connects directly.
func New(timeout time.Duration) *http.Client {
	transport := newTransport()
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// NewWithTransport creates an *http.Client with the given timeout and transport.
// Callers can wrap the shared proxy-aware transport while still centralizing
// timeout handling in this package.
func NewWithTransport(timeout time.Duration, transport http.RoundTripper) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// DefaultTransport returns a proxy-aware *http.Transport for cases where
// callers need to wrap it (e.g. Bybit's headerRoundTripper).
func DefaultTransport() *http.Transport {
	return newTransport()
}

func newTransport() *http.Transport {
	return &http.Transport{
		Proxy:               resolveProxy,
		DialContext:         dialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	}
}

// resolveProxy returns the proxy URL for the given request.
// Checks ALL_PROXY, HTTPS_PROXY, HTTP_PROXY in that order.
func resolveProxy(req *http.Request) (*url.URL, error) {
	// Determine which env var to use based on the request scheme
	var envKey string
	if req.URL.Scheme == "https" {
		envKey = "HTTPS_PROXY"
	} else {
		envKey = "HTTP_PROXY"
	}

	// ALL_PROXY overrides per-scheme settings
	if v := getEnv("ALL_PROXY", "all_proxy"); v != "" {
		return parseProxyURL(v)
	}
	if v := getEnv(envKey, strings.ToLower(envKey)); v != "" {
		return parseProxyURL(v)
	}
	return nil, nil
}

// dialContext creates a net.Conn, routing through a SOCKS5 proxy if configured.
// This is needed because Go's stdlib http.ProxyFromEnvironment does not support
// SOCKS5 — it only handles HTTP/HTTPS CONNECT proxies. The resolveProxy function
// handles HTTP/HTTPS proxies via the stdlib mechanism, while dialContext handles
// SOCKS5 by creating a custom dialer.
func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	proxyURL := findProxyURL()
	if proxyURL == nil {
		return baseNetDialer().DialContext(ctx, network, addr)
	}

	scheme := strings.ToLower(proxyURL.Scheme)
	if scheme != "socks5" && scheme != "socks" && scheme != "socks5h" {
		// Not a SOCKS proxy — fall through to direct dial.
		// HTTP/HTTPS CONNECT proxies are handled by resolveProxy above.
		return baseNetDialer().DialContext(ctx, network, addr)
	}

	dialer, err := newSOCKS5Dialer(proxyURL)
	if err != nil {
		return nil, err
	}
	return dialer.(proxy.ContextDialer).DialContext(ctx, network, addr)
}

func baseNetDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
}

// findProxyURL finds the effective proxy URL from environment variables.
func findProxyURL() *url.URL {
	if v := getEnv("ALL_PROXY", "all_proxy"); v != "" {
		if u, err := parseProxyURL(v); err == nil && u != nil {
			return u
		}
	}
	// Check both HTTPS_PROXY and HTTP_PROXY (case-insensitive)
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if v := os.Getenv(key); v != "" {
			if u, err := parseProxyURL(v); err == nil && u != nil {
				return u
			}
		}
	}
	return nil
}

func getEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func parseProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// newSOCKS5Dialer creates a SOCKS5 dialer from a proxy URL.
func newSOCKS5Dialer(proxyURL *url.URL) (proxy.Dialer, error) {
	var auth *proxy.Auth
	if proxyURL.User != nil {
		auth = &proxy.Auth{
			User: proxyURL.User.Username(),
		}
		if pass, ok := proxyURL.User.Password(); ok {
			auth.Password = pass
		}
	}

	host := proxyURL.Host
	if !strings.Contains(host, ":") {
		host = host + ":1080"
	}

	return proxy.SOCKS5("tcp", host, auth, proxy.Direct)
}

// SetProxyEnv logs the active proxy configuration.
// Call this early in main() so SDK-based clients (Hyperliquid etc.)
// pick up the proxy via Go's http.ProxyFromEnvironment.
func SetProxyEnv() {
	proxyURL := findProxyURL()
	if proxyURL == nil {
		return
	}

	scheme := strings.ToLower(proxyURL.Scheme)
	switch scheme {
	case "socks5", "socks", "socks5h":
		log.Printf("🔒 Proxy configured: socks5://%s", proxyURL.Host)
	case "http", "https":
		log.Printf("🔒 Proxy configured: %s://%s", scheme, proxyURL.Host)
	default:
		log.Printf("⚠️  Unsupported proxy scheme: %s (supported: socks5, socks, http, https)", scheme)
	}
}

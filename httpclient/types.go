package httpclient

import (
	"net/http"
	"net/url"
)

// Type aliases re-exported from net/http so callers can depend solely on
// fxcore/httpclient for both construction and types. These are aliases (not
// wrapper types): GC-compatible with any *http.Client / *http.Request /
// *http.Response obtained elsewhere.
type (
	// Client aliases *http.Client.
	Client = http.Client
	// Request aliases *http.Request.
	Request = http.Request
	// Response aliases *http.Response.
	Response = http.Response
	// Header aliases http.Header.
	Header = http.Header
	// Transport aliases *http.Transport.
	Transport = http.Transport
	// RoundTripper mirrors http.RoundTripper.
	RoundTripper = http.RoundTripper
	// URL mirrors *url.URL.
	URL = url.URL
)

// HTTP verbs mirrored from net/http.Method* constants.
const (
	MethodGet     = http.MethodGet
	MethodHead    = http.MethodHead
	MethodPost    = http.MethodPost
	MethodPut     = http.MethodPut
	MethodPatch   = http.MethodPatch
	MethodDelete  = http.MethodDelete
	MethodConnect = http.MethodConnect
	MethodOptions = http.MethodOptions
	MethodTrace   = http.MethodTrace
)

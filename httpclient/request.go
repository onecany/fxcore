package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// NewRequest builds an HTTP request with optional headers.
func NewRequest(ctx context.Context, method, requestURL string, body io.Reader, headers http.Header) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, err
	}
	applyHeaders(req.Header, headers)
	return req, nil
}

// NewJSONRequest marshals payload as JSON, creates the request, and ensures the
// Content-Type is application/json unless the caller already set one.
func NewJSONRequest(ctx context.Context, method, requestURL string, payload any, headers http.Header) (*http.Request, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal json body: %w", err)
	}
	req, err := NewRequest(ctx, method, requestURL, bytes.NewReader(body), headers)
	if err != nil {
		return nil, err
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// Do creates and executes a request with the provided client.
func Do(ctx context.Context, client *http.Client, method, requestURL string, body io.Reader, headers http.Header) (*http.Response, error) {
	if client == nil {
		return nil, fmt.Errorf("http client is required")
	}
	req, err := NewRequest(ctx, method, requestURL, body, headers)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// DoJSON marshals payload as JSON, creates the request, and executes it with
// the provided client.
func DoJSON(ctx context.Context, client *http.Client, method, requestURL string, payload any, headers http.Header) (*http.Response, error) {
	if client == nil {
		return nil, fmt.Errorf("http client is required")
	}
	req, err := NewJSONRequest(ctx, method, requestURL, payload, headers)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func applyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

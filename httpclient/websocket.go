package httpclient

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

// DialWebsocketContext opens a WebSocket connection using the shared proxy-aware
// dialing logic from this package.
func DialWebsocketContext(ctx context.Context, serverURL, origin string, protocols ...string) (*websocket.Conn, error) {
	config, err := websocket.NewConfig(serverURL, origin)
	if err != nil {
		return nil, err
	}
	if len(protocols) > 0 {
		config.Protocol = append([]string(nil), protocols...)
	}
	return DialWebsocketConfigContext(ctx, config)
}

// DialWebsocketConfigContext opens a WebSocket connection from a prebuilt config
// while honoring proxy configuration from ALL_PROXY / HTTPS_PROXY / HTTP_PROXY.
func DialWebsocketConfigContext(ctx context.Context, config *websocket.Config) (*websocket.Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if config == nil {
		return nil, fmt.Errorf("websocket config is required")
	}
	if config.Location == nil {
		return nil, &websocket.DialError{Config: config, Err: websocket.ErrBadWebSocketLocation}
	}
	if config.Origin == nil {
		return nil, &websocket.DialError{Config: config, Err: websocket.ErrBadWebSocketOrigin}
	}

	conn, err := dialWebsocketConn(ctx, config)
	if err != nil {
		return nil, &websocket.DialError{Config: config, Err: err}
	}

	success := false
	defer func() {
		if !success {
			_ = conn.Close()
		}
	}()

	var ws *websocket.Conn
	var wsErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		ws, wsErr = websocket.NewClient(config, conn)
	}()

	select {
	case <-ctx.Done():
		_ = conn.SetDeadline(time.Now())
		_ = conn.Close()
		<-done
		return nil, &websocket.DialError{Config: config, Err: ctx.Err()}
	case <-done:
		if wsErr != nil {
			return nil, &websocket.DialError{Config: config, Err: wsErr}
		}
		success = true
		return ws, nil
	}
}

func dialWebsocketConn(ctx context.Context, config *websocket.Config) (net.Conn, error) {
	targetAddr := websocketTargetAddress(config.Location)
	proxyURL := findProxyURL()

	var (
		conn net.Conn
		err  error
	)

	if proxyURL != nil {
		switch strings.ToLower(proxyURL.Scheme) {
		case "http", "https":
			conn, err = dialHTTPProxyTunnel(ctx, proxyURL, targetAddr)
		default:
			conn, err = dialContext(ctx, "tcp", targetAddr)
		}
	} else {
		conn, err = dialContext(ctx, "tcp", targetAddr)
	}
	if err != nil {
		return nil, err
	}

	if config.Location.Scheme != "wss" {
		return conn, nil
	}

	tlsConfig := config.TlsConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	} else {
		tlsConfig = tlsConfig.Clone()
	}
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = config.Location.Hostname()
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func websocketTargetAddress(location *url.URL) string {
	host := location.Host
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}

	switch strings.ToLower(location.Scheme) {
	case "wss":
		return net.JoinHostPort(host, "443")
	default:
		return net.JoinHostPort(host, "80")
	}
}

func dialHTTPProxyTunnel(ctx context.Context, proxyURL *url.URL, targetAddr string) (net.Conn, error) {
	proxyAddr := proxyAddress(proxyURL)
	conn, err := baseNetDialer().DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	if strings.EqualFold(proxyURL.Scheme, "https") {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: proxyURL.Hostname()})
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		conn = tlsConn
	}

	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: targetAddr},
		Host:   targetAddr,
		Header: make(http.Header),
	}
	if header := proxyAuthorizationHeader(proxyURL); header != "" {
		req.Header.Set("Proxy-Authorization", header)
	}
	if err = req.Write(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		_ = resp.Body.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("proxy CONNECT failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return &bufferedConn{Conn: conn, reader: reader}, nil
}

func proxyAddress(proxyURL *url.URL) string {
	host := proxyURL.Host
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	if strings.EqualFold(proxyURL.Scheme, "https") {
		return net.JoinHostPort(host, "443")
	}
	return net.JoinHostPort(host, "80")
}

func proxyAuthorizationHeader(proxyURL *url.URL) string {
	if proxyURL == nil || proxyURL.User == nil {
		return ""
	}
	password, _ := proxyURL.User.Password()
	token := proxyURL.User.Username() + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(token))
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

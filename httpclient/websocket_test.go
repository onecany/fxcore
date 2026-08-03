package httpclient

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestDialWebsocketContextDirect(t *testing.T) {
	wsServer := newWebsocketTestServer(t)
	defer wsServer.Close()

	conn, err := DialWebsocketContext(context.Background(), websocketURL(wsServer.URL), "http://localhost/")
	if err != nil {
		t.Fatalf("DialWebsocketContext() error = %v", err)
	}
	defer conn.Close()

	if err := websocket.Message.Send(conn, "hello"); err != nil {
		t.Fatalf("send websocket message: %v", err)
	}

	var got string
	if err := websocket.Message.Receive(conn, &got); err != nil {
		t.Fatalf("receive websocket message: %v", err)
	}
	if got != "echo:hello" {
		t.Fatalf("unexpected websocket response: %q", got)
	}
}

func TestDialWebsocketContextViaHTTPProxy(t *testing.T) {
	wsServer := newWebsocketTestServer(t)
	defer wsServer.Close()

	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}

		targetConn, err := net.DialTimeout("tcp", r.Host, 5*time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			targetConn.Close()
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}

		clientConn, rw, err := hijacker.Hijack()
		if err != nil {
			targetConn.Close()
			return
		}

		if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			clientConn.Close()
			targetConn.Close()
			return
		}
		if err := rw.Flush(); err != nil {
			clientConn.Close()
			targetConn.Close()
			return
		}

		go proxyCopyBothWays(clientConn, targetConn)
	}))
	defer proxyServer.Close()

	t.Setenv("ALL_PROXY", "")
	t.Setenv("all_proxy", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("https_proxy", "")
	t.Setenv("HTTP_PROXY", proxyServer.URL)
	t.Setenv("http_proxy", proxyServer.URL)

	conn, err := DialWebsocketContext(context.Background(), websocketURL(wsServer.URL), "http://localhost/")
	if err != nil {
		t.Fatalf("DialWebsocketContext() via proxy error = %v", err)
	}
	defer conn.Close()

	if err := websocket.Message.Send(conn, "proxy"); err != nil {
		t.Fatalf("send websocket message through proxy: %v", err)
	}

	var got string
	if err := websocket.Message.Receive(conn, &got); err != nil {
		t.Fatalf("receive websocket message through proxy: %v", err)
	}
	if got != "echo:proxy" {
		t.Fatalf("unexpected proxied websocket response: %q", got)
	}
}

func newWebsocketTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		websocket.Handler(func(conn *websocket.Conn) {
			defer conn.Close()
			var msg string
			if err := websocket.Message.Receive(conn, &msg); err != nil {
				return
			}
			_ = websocket.Message.Send(conn, "echo:"+msg)
		}).ServeHTTP(w, r)
	}))
}

func websocketURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func proxyCopyBothWays(left, right net.Conn) {
	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(left, right)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(right, left)
		done <- struct{}{}
	}()

	<-done
	_ = left.Close()
	_ = right.Close()
	<-done
}

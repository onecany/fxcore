package exchange

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestBinanceSignedRequest 验证签名 query 构造（用 mock server 捕获请求）。
func TestBinanceSignedRequest(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"asset":"USDT","availableBalance":"100.5"}]`))
	}))
	defer srv.Close()

	adapter, err := newBinance(Credentials{
		ExchangeType: "binance",
		APIKey:       "test-key",
		SecretKey:    "test-secret",
		BaseURL:      srv.URL,
	})
	if err != nil {
		t.Fatalf("newBinance: %v", err)
	}
	bal, err := adapter.GetBalance(t.Context())
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal != 100.5 {
		t.Errorf("balance: %v", bal)
	}
	// 签名头
	if !strings.Contains(capturedURL, "timestamp=") {
		t.Errorf("missing timestamp: %s", capturedURL)
	}
	if !strings.Contains(capturedURL, "signature=") {
		t.Errorf("missing signature: %s", capturedURL)
	}
	if !strings.Contains(capturedURL, "recvWindow=5000") {
		t.Errorf("missing recvWindow: %s", capturedURL)
	}
}

// TestBinanceSignedOrder 验证下单 payload 与幂等键。
func TestBinanceSignedOrder(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "exchangeInfo"):
			_, _ = w.Write([]byte(`{"symbols":[{"symbol":"BTCUSDT","filters":[{"filterType":"LOT_SIZE","stepSize":"0.001"}]}]}`))
		case strings.Contains(r.URL.Path, "leverage"):
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`{"orderId":12345,"clientOrderId":"cid-1","status":"FILLED","executedQty":"1.5","avgPrice":"100"}`))
		}
	}))
	defer srv.Close()

	adapter, err := newBinance(Credentials{
		ExchangeType: "binance", APIKey: "k", SecretKey: "s", BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("newBinance: %v", err)
	}
	res, err := adapter.OpenLong(t.Context(), OrderParams{
		Symbol: "BTCUSDT", Quantity: 1.5, Leverage: 5, ClientOrderID: "trader-1-1-BTCUSDT-open_long",
	})
	if err != nil {
		t.Fatalf("OpenLong: %v", err)
	}
	if res.ExchangeOrderID != "12345" || res.Status != "FILLED" {
		t.Errorf("result: %+v", res)
	}
	for _, want := range []string{"side=BUY", "positionSide=LONG", "newClientOrderId=trader-1-1-BTCUSDT-open_long", "type=MARKET"} {
		if !strings.Contains(gotURL, want) {
			t.Errorf("order url missing %s: %s", want, gotURL)
		}
	}
}

// TestBinanceReduceOnlyClose 平仓带 reduceOnly。
func TestBinanceReduceOnlyClose(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "exchangeInfo"):
			_, _ = w.Write([]byte(`{"symbols":[{"symbol":"BTCUSDT","filters":[{"filterType":"LOT_SIZE","stepSize":"0.001"}]}]}`))
		case strings.Contains(r.URL.Path, "leverage"):
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`{"orderId":1,"status":"NEW"}`))
		}
	}))
	defer srv.Close()
	adapter, _ := newBinance(Credentials{ExchangeType: "binance", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	_, err := adapter.CloseLong(t.Context(), OrderParams{Symbol: "BTCUSDT", Quantity: 1})
	if err != nil {
		t.Fatalf("CloseLong: %v", err)
	}
	if !strings.Contains(gotURL, "reduceOnly=true") {
		t.Errorf("missing reduceOnly: %s", gotURL)
	}
}

// TestBinanceErrorMapping 4xx 归 ErrConn。
func TestBinanceErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":-4120,"msg":"STOP_ORDER"}`))
	}))
	defer srv.Close()
	adapter, _ := newBinance(Credentials{ExchangeType: "binance", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	if _, err := adapter.GetBalance(t.Context()); err == nil {
		t.Errorf("expected error")
	}
}

var _ = time.Now
var _ = context.Background

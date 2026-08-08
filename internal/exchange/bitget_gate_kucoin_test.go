package exchange

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestBitgetSignedHeaders 验证 ACCESS-* 四头。
func TestBitgetSignedHeaders(t *testing.T) {
	var gotSig, gotTs, gotKey, gotPass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("ACCESS-SIGN")
		gotTs = r.Header.Get("ACCESS-TIMESTAMP")
		gotKey = r.Header.Get("ACCESS-KEY")
		gotPass = r.Header.Get("ACCESS-PASSPHRASE")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "contracts"):
			_, _ = w.Write([]byte(`{"code":"00000","data":[{"symbol":"BTCUSDT","volumePlace":3}]}`))
		case strings.Contains(r.URL.Path, "set-leverage"):
			_, _ = w.Write([]byte(`{"code":"00000","data":{}}`))
		default:
			_, _ = w.Write([]byte(`{"code":"00000","data":{"orderId":"bg-1"},"msg":""}`))
		}
	}))
	defer srv.Close()

	adapter, err := newBitget(Credentials{ExchangeType: "bitget", APIKey: "k", SecretKey: "s", Passphrase: "pp", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newBitget: %v", err)
	}
	res, err := adapter.OpenLong(t.Context(), OrderParams{Symbol: "BTCUSDT", Quantity: 1.234, Leverage: 5, ClientOrderID: "cid"})
	if err != nil {
		t.Fatalf("OpenLong: %v", err)
	}
	if res.ExchangeOrderID != "bg-1" {
		t.Errorf("order id: %s", res.ExchangeOrderID)
	}
	if gotKey != "k" || gotPass != "pp" || gotTs == "" || gotSig == "" {
		t.Errorf("headers missing: key=%s pass=%s ts=%s sig=%s", gotKey, gotPass, gotTs, gotSig)
	}
}

// TestBitgetVolumePlace 数量 floor 到小数位。
func TestBitgetVolumePlace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "contracts") {
			_, _ = w.Write([]byte(`{"code":"00000","data":[{"symbol":"BTCUSDT","volumePlace":3}]}`))
		} else {
			_, _ = w.Write([]byte(`{"code":"00000","data":{"orderId":"1"},"msg":""}`))
		}
	}))
	defer srv.Close()
	adapter, _ := newBitget(Credentials{ExchangeType: "bitget", APIKey: "k", SecretKey: "s", Passphrase: "p", BaseURL: srv.URL})
	qty, err := adapter.FormatQuantity(t.Context(), "BTCUSDT", 1.23456)
	if err != nil || qty != 1.234 {
		t.Errorf("format: %v %v", qty, err)
	}
}

// TestGateSignedHeaders 验证 KEY/SIGN/Timestamp 三头。
func TestGateSignedHeaders(t *testing.T) {
	var gotSig, gotTs, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("SIGN")
		gotTs = r.Header.Get("Timestamp")
		gotKey = r.Header.Get("KEY")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"status":"open","size":10,"left":10,"fill_price":"0"}`))
	}))
	defer srv.Close()

	adapter, err := newGate(Credentials{ExchangeType: "gate", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newGate: %v", err)
	}
	res, err := adapter.OpenLong(t.Context(), OrderParams{Symbol: "BTC_USDT", Quantity: 3.7})
	if err != nil {
		t.Fatalf("OpenLong: %v", err)
	}
	if res.ExchangeOrderID != "123" {
		t.Errorf("order id: %s", res.ExchangeOrderID)
	}
	if gotKey != "k" || gotTs == "" || gotSig == "" {
		t.Errorf("headers missing: key=%s ts=%s sig=%s", gotKey, gotTs, gotSig)
	}
}

// TestGateSizeSign 正负 size 表示方向（开多正、开空负）。
func TestGateSizeSign(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodies = append(bodies, readAllBody(r))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"status":"open","size":10,"left":10,"fill_price":"0"}`))
	}))
	defer srv.Close()
	adapter, _ := newGate(Credentials{ExchangeType: "gate", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	_, _ = adapter.OpenLong(t.Context(), OrderParams{Symbol: "BTC_USDT", Quantity: 5})
	_, _ = adapter.OpenShort(t.Context(), OrderParams{Symbol: "BTC_USDT", Quantity: 5})
	if len(bodies) != 2 {
		t.Fatalf("bodies: %d", len(bodies))
	}
	if !strings.Contains(bodies[0], `"size":5`) {
		t.Errorf("open long should be +5: %s", bodies[0])
	}
	if !strings.Contains(bodies[1], `"size":-5`) {
		t.Errorf("open short should be -5: %s", bodies[1])
	}
}

// TestGateReduceOnly 平仓带 reduce_only。
func TestGateReduceOnly(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = readAllBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"status":"open","size":10,"left":10,"fill_price":"0"}`))
	}))
	defer srv.Close()
	adapter, _ := newGate(Credentials{ExchangeType: "gate", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	_, err := adapter.CloseLong(t.Context(), OrderParams{Symbol: "BTC_USDT", Quantity: 5})
	if err != nil {
		t.Fatalf("CloseLong: %v", err)
	}
	if !strings.Contains(gotBody, `"reduce_only":true`) {
		t.Errorf("missing reduce_only: %s", gotBody)
	}
}

// TestKuCoinSignedHeaders 验证 KC-API-* 四头。
func TestKuCoinSignedHeaders(t *testing.T) {
	var gotSig, gotTs, gotKey, gotPass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("KC-API-SIGN")
		gotTs = r.Header.Get("KC-API-TIMESTAMP")
		gotKey = r.Header.Get("KC-API-KEY")
		gotPass = r.Header.Get("KC-API-PASSPHRASE")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "contracts"):
			_, _ = w.Write([]byte(`{"code":"200000","data":[{"symbol":"XBTUSDTM","baseMinSize":"0.001"}]}`))
		case strings.Contains(r.URL.Path, "leverage"):
			_, _ = w.Write([]byte(`{"code":"200000","data":{}}`))
		default:
			_, _ = w.Write([]byte(`{"code":"200000","data":{"orderId":"kc-1"}}`))
		}
	}))
	defer srv.Close()

	adapter, err := newKuCoin(Credentials{ExchangeType: "kucoin", APIKey: "k", SecretKey: "s", Passphrase: "pp", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newKuCoin: %v", err)
	}
	res, err := adapter.OpenLong(t.Context(), OrderParams{Symbol: "XBTUSDTM", Quantity: 1, Leverage: 5, ClientOrderID: "cid"})
	if err != nil {
		t.Fatalf("OpenLong: %v", err)
	}
	if res.ExchangeOrderID != "kc-1" {
		t.Errorf("order id: %s", res.ExchangeOrderID)
	}
	if gotKey != "k" || gotPass != "pp" || gotTs == "" || gotSig == "" {
		t.Errorf("headers missing: key=%s pass=%s ts=%s sig=%s", gotKey, gotPass, gotTs, gotSig)
	}
}

// TestKuCoinCloseOrder 平仓带 closeOrder。
func TestKuCoinCloseOrder(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = readAllBody(r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "contracts"):
			_, _ = w.Write([]byte(`{"code":"200000","data":[{"symbol":"XBTUSDTM","baseMinSize":"0.001"}]}`))
		case strings.Contains(r.URL.Path, "leverage"):
			_, _ = w.Write([]byte(`{"code":"200000","data":{}}`))
		default:
			_, _ = w.Write([]byte(`{"code":"200000","data":{"orderId":"1"}}`))
		}
	}))
	defer srv.Close()
	adapter, _ := newKuCoin(Credentials{ExchangeType: "kucoin", APIKey: "k", SecretKey: "s", Passphrase: "p", BaseURL: srv.URL})
	_, err := adapter.CloseLong(t.Context(), OrderParams{Symbol: "XBTUSDTM", Quantity: 1, Leverage: 5})
	if err != nil {
		t.Fatalf("CloseLong: %v", err)
	}
	if !strings.Contains(gotBody, `"closeOrder":true`) {
		t.Errorf("missing closeOrder: %s", gotBody)
	}
}

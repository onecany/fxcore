package exchange

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// readAllBody 读取请求体。
func readAllBody(r *http.Request) string {
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

// TestBybitSignedHeaders 验证 X-BAPI-* 签名头。
func TestBybitSignedHeaders(t *testing.T) {
	var gotSig, gotTS, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-BAPI-SIGN")
		gotTS = r.Header.Get("X-BAPI-TIMESTAMP")
		gotKey = r.Header.Get("X-BAPI-API-KEY")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "instruments-info"):
			_, _ = w.Write([]byte(`{"retCode":0,"result":{"list":[{"symbol":"BTCUSDT","lotSizeFilter":{"qtyStep":"0.001"}}]}}`))
		case strings.Contains(r.URL.Path, "set-leverage"):
			_, _ = w.Write([]byte(`{"retCode":0}`))
		default:
			_, _ = w.Write([]byte(`{"retCode":0,"result":{"orderId":"bybit-1","orderStatus":"New"}}`))
		}
	}))
	defer srv.Close()

	adapter, err := newBybit(Credentials{ExchangeType: "bybit", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newBybit: %v", err)
	}
	res, err := adapter.OpenLong(t.Context(), OrderParams{Symbol: "BTCUSDT", Quantity: 1, Leverage: 5, ClientOrderID: "cid-x"})
	if err != nil {
		t.Fatalf("OpenLong: %v", err)
	}
	if res.ExchangeOrderID != "bybit-1" {
		t.Errorf("order id: %s", res.ExchangeOrderID)
	}
	if gotKey != "k" || gotTS == "" || gotSig == "" {
		t.Errorf("headers missing: key=%s ts=%s sig=%s", gotKey, gotTS, gotSig)
	}
}

// TestBybitCloseReduceOnly 平仓带 reduceOnly。
func TestBybitCloseReduceOnly(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = readAllBody(r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "instruments-info"):
			_, _ = w.Write([]byte(`{"retCode":0,"result":{"list":[{"symbol":"BTCUSDT","lotSizeFilter":{"qtyStep":"0.001"}}]}}`))
		case strings.Contains(r.URL.Path, "set-leverage"):
			_, _ = w.Write([]byte(`{"retCode":0}`))
		default:
			_, _ = w.Write([]byte(`{"retCode":0,"result":{"orderId":"1","orderStatus":"New"}}`))
		}
	}))
	defer srv.Close()
	adapter, _ := newBybit(Credentials{ExchangeType: "bybit", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	_, err := adapter.CloseLong(t.Context(), OrderParams{Symbol: "BTCUSDT", Quantity: 1})
	if err != nil {
		t.Fatalf("CloseLong: %v", err)
	}
	for _, want := range []string{`"side":"Sell"`, `"reduceOnly":true`, `"positionIdx":1`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("body missing %s: %s", want, gotBody)
		}
	}
}

// TestBybitGetFills 成交解析。
func TestBybitGetFills(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"retCode":0,"result":{"list":[{"execId":"exec-1","orderId":"o-1","symbol":"BTCUSDT","side":"Buy","execPrice":"50000","execQty":"0.1","execFee":"0.5","feeCurrency":"USDT","execTime":"1754000000000"}]}}`))
	}))
	defer srv.Close()
	adapter, _ := newBybit(Credentials{ExchangeType: "bybit", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	fills, err := adapter.GetFills(t.Context(), 0)
	if err != nil {
		t.Fatalf("GetFills: %v", err)
	}
	if len(fills) != 1 || fills[0].ExchangeTradeID != "exec-1" || fills[0].Price != 50000 {
		t.Errorf("fills: %+v", fills)
	}
}

// TestOKXSignedHeaders 验证 OK-ACCESS-* 头（含 passphrase）。
func TestOKXSignedHeaders(t *testing.T) {
	var gotSig, gotTs, gotKey, gotPass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("OK-ACCESS-SIGN")
		gotTs = r.Header.Get("OK-ACCESS-TIMESTAMP")
		gotKey = r.Header.Get("OK-ACCESS-KEY")
		gotPass = r.Header.Get("OK-ACCESS-PASSPHRASE")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","data":[{"ordId":"okx-1","clOrdId":"cid","sCode":"0","sMsg":""}]}`))
	}))
	defer srv.Close()

	adapter, err := newOKX(Credentials{ExchangeType: "okx", APIKey: "k", SecretKey: "s", Passphrase: "pp", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newOKX: %v", err)
	}
	res, err := adapter.OpenLong(t.Context(), OrderParams{Symbol: "BTC-USDT-SWAP", Quantity: 1, Leverage: 5})
	if err != nil {
		t.Fatalf("OpenLong: %v", err)
	}
	if res.ExchangeOrderID != "okx-1" {
		t.Errorf("order id: %s", res.ExchangeOrderID)
	}
	if gotKey != "k" || gotPass != "pp" || gotTs == "" || gotSig == "" {
		t.Errorf("headers missing: key=%s pass=%s ts=%s sig=%s", gotKey, gotPass, gotTs, gotSig)
	}
}

// TestOKXIsMakerExecType 成交 IsMaker 判定（ExecType==M）。
func TestOKXIsMakerExecType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","data":[{"instId":"BTC-USDT-SWAP","billId":"b1","ordId":"o1","side":"buy","px":"50000","sz":"1","fee":"0.5","feeCcy":"USDT","pnl":"0","execType":"M","ts":"1754000000000"}]}`))
	}))
	defer srv.Close()
	adapter, _ := newOKX(Credentials{ExchangeType: "okx", APIKey: "k", SecretKey: "s", Passphrase: "p", BaseURL: srv.URL})
	fills, err := adapter.GetFills(t.Context(), 0)
	if err != nil {
		t.Fatalf("GetFills: %v", err)
	}
	if len(fills) != 1 || !fills[0].IsMaker {
		t.Errorf("IsMaker should be true for ExecType M: %+v", fills)
	}
}

// TestOKXAlgoSLTP attachAlgoOrds 参数。
func TestOKXAlgoSLTP(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = readAllBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","data":[{"ordId":"1","clOrdId":"","sCode":"0","sMsg":""}]}`))
	}))
	defer srv.Close()
	adapter, _ := newOKX(Credentials{ExchangeType: "okx", APIKey: "k", SecretKey: "s", Passphrase: "p", BaseURL: srv.URL})
	_, err := adapter.OpenLong(t.Context(), OrderParams{Symbol: "BTC-USDT-SWAP", Quantity: 1, StopLoss: 48000, TakeProfit: 52000})
	if err != nil {
		t.Fatalf("OpenLong with algo: %v", err)
	}
	for _, want := range []string{`"slTriggerPx":"48000"`, `"tpTriggerPx":"52000"`, `"tdMode":"cross"`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("body missing %s: %s", want, gotBody)
		}
	}
}

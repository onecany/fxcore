package exchange

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// readAllBody 读取请求体。
func readAllBody(r *http.Request) string {
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

// hmacSHA256Hex 重算 Bybit 签名（hex(HMAC-SHA256(ts+apiKey+recv+content), secret)）。
func hmacSHA256Hex(secret, content string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(content))
	return hex.EncodeToString(mac.Sum(nil))
}

// hmacSHA256B64 重算 OKX/KuCoin 签名（Base64(HMAC-SHA256(ts+method+path+body), secret)）。
func hmacSHA256B64(secret, content string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(content))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// TestBybitSignedHeaders 验证 X-BAPI-* 签名头（签名串须含 api_key，Bybit v5 官方规范）。
func TestBybitSignedHeaders(t *testing.T) {
	var gotSig, gotTS, gotKey string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-BAPI-SIGN")
		gotTS = r.Header.Get("X-BAPI-TIMESTAMP")
		gotKey = r.Header.Get("X-BAPI-API-KEY")
		gotBody = readAllBody(r)
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
	// 签名串 = ts + api_key + recv_window + raw body（官方规范），缺 api_key 会报 10004 sign error
	want := hmacSHA256Hex("s", gotTS+"k"+"5000"+gotBody)
	if gotSig != want {
		t.Errorf("X-BAPI-SIGN mismatch: got %s want %s (signature must include api_key)", gotSig, want)
	}
}

// TestBybitGetQuerySign GET 签名须含 query string（官方：ts+api_key+recv_window+queryString）。
func TestBybitGetQuerySign(t *testing.T) {
	var gotSig, gotTS string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-BAPI-SIGN")
		gotTS = r.Header.Get("X-BAPI-TIMESTAMP")
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "wallet-balance") {
			_, _ = w.Write([]byte(`{"retCode":0,"result":{"list":[{"totalEquity":"100","coin":[{"coin":"USDT","walletBalance":"75.94"}]}]}}`))
		} else {
			_, _ = w.Write([]byte(`{"retCode":0,"result":{"list":[]}}`))
		}
	}))
	defer srv.Close()
	adapter, _ := newBybit(Credentials{ExchangeType: "bybit", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	if _, err := adapter.GetBalance(t.Context()); err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	want := hmacSHA256Hex("s", gotTS+"k"+"5000"+"accountType=UNIFIED")
	if gotSig != want {
		t.Errorf("GET signature must include query string: got %s want %s", gotSig, want)
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

// TestOKXSignedHeaders 验证 OK-ACCESS-* 头（含 passphrase）+ 签名可复算。
func TestOKXSignedHeaders(t *testing.T) {
	var gotSig, gotTs, gotKey, gotPass string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("OK-ACCESS-SIGN")
		gotTs = r.Header.Get("OK-ACCESS-TIMESTAMP")
		gotKey = r.Header.Get("OK-ACCESS-KEY")
		gotPass = r.Header.Get("OK-ACCESS-PASSPHRASE")
		gotBody = readAllBody(r)
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
	// OK-ACCESS-TIMESTAMP 必须 ISO 8601 UTC（如 2020-12-08T09:08:57.715Z）；
	// 纯数字（毫秒）会被 OKX 判 50102 Timestamp request expired（回归锁定）。
	if _, err := time.Parse("2006-01-02T15:04:05.000Z", gotTs); err != nil {
		t.Errorf("OK-ACCESS-TIMESTAMP 不是 ISO 8601 UTC 格式: %q", gotTs)
	}
	// 签名 = Base64(HMAC-SHA256(ts + method + requestPath + body))（官方规范）
	want := hmacSHA256B64("s", gotTs+"POST"+"/api/v5/trade/order"+gotBody)
	if gotSig != want {
		t.Errorf("OK-ACCESS-SIGN mismatch: got %s want %s", gotSig, want)
	}
}

// TestOKXQueryGetSign 带 query 的 GET 签名必须含完整 requestPath（官方：/api/v5/account/balance?ccy=BTC）。
// 回归锁定 50113 Invalid Sign：砍掉 query 再签名会让 orders-pending/fills-history 等带参 GET 全挂。
func TestOKXQueryGetSign(t *testing.T) {
	var gotSig, gotTs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("OK-ACCESS-SIGN")
		gotTs = r.Header.Get("OK-ACCESS-TIMESTAMP")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","data":[]}`))
	}))
	defer srv.Close()
	adapter, _ := newOKX(Credentials{ExchangeType: "okx", APIKey: "k", SecretKey: "s", Passphrase: "p", BaseURL: srv.URL})
	if _, err := adapter.GetOpenOrders(t.Context()); err != nil {
		t.Fatalf("GetOpenOrders: %v", err)
	}
	// requestPath 必须含 ?instType=SWAP
	want := hmacSHA256B64("s", gotTs+"GET"+"/api/v5/trade/orders-pending?instType=SWAP")
	if gotSig != want {
		t.Errorf("GET signature must include full requestPath with query: got %s want %s", gotSig, want)
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

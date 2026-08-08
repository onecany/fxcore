package exchange

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestKeccak16 已知向量（LegacyKeccak256 空输入）。
func TestKeccak16(t *testing.T) {
	// keccak256("") = c5d246...；前 16 字节
	got := hex.EncodeToString(keccak16([]byte("")))
	if got != "c5d2460186f7233c927e7db2dcc703c0" {
		t.Errorf("keccak16 empty: %s", got)
	}
	// keccak256("abc") = 4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45
	got = hex.EncodeToString(keccak16([]byte("abc")))
	if got != "4e03657aea45a94fc7d47ba826c8d667" {
		t.Errorf("keccak16 abc: %s", got)
	}
}

// TestHLL1SignMessage 验证消息构造（0x00 + hash16 + nonce8 + vault32 = 57 字节）。
func TestHLL1SignMessage(t *testing.T) {
	seed := make([]byte, 32) // 全零 seed
	for i := range seed {
		seed[i] = byte(i)
	}
	a := &HyperliquidAdapter{creds: Credentials{PrivateKey: hex.EncodeToString(seed)}}
	action := map[string]any{"type": "order", "orders": []any{}}
	sig, err := a.l1Sign(action, 12345)
	if err != nil {
		t.Fatalf("l1Sign: %v", err)
	}
	if len(sig.R) != 64 || len(sig.S) != 64 {
		t.Errorf("sig hex length: r=%d s=%d", len(sig.R), len(sig.S))
	}
	// 确定性：同输入同输出
	sig2, _ := a.l1Sign(action, 12345)
	if sig.R != sig2.R || sig.S != sig2.S {
		t.Errorf("signature not deterministic")
	}
}

// TestHLNoPrivateKey 无私钥下单拒绝。
func TestHLNoPrivateKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"universe":[{"name":"BTC"}]}`))
	}))
	defer srv.Close()
	adapter, err := newHyperliquid(Credentials{
		ExchangeType: "hyperliquid",
		WalletAddr:   "0x1234",
		PrivateKey:   "",
		BaseURL:      srv.URL,
	})
	if err != nil {
		t.Fatalf("newHyperliquid: %v", err)
	}
	if _, err := adapter.OpenLong(t.Context(), OrderParams{Symbol: "BTC-USDT", Quantity: 0.01}); err == nil {
		t.Errorf("expected error without private key")
	}
}

// TestHLWalletRequired 无钱包地址构造拒绝。
func TestHLWalletRequired(t *testing.T) {
	if _, err := newHyperliquid(Credentials{ExchangeType: "hyperliquid"}); err == nil {
		t.Errorf("expected error without wallet address")
	}
}

// TestHLPlaceOrder 验证 /exchange body 结构（action/nonce/signature）。
func TestHLPlaceOrder(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = readAllBody(r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/info"):
			if strings.Contains(gotBody, `"type":"meta"`) {
				_, _ = w.Write([]byte(`{"universe":[{"name":"BTC"},{"name":"ETH"}]}`))
			} else if strings.Contains(gotBody, `"type":"metaAndAssetCtxs"`) {
				_, _ = w.Write([]byte(`[{"midPx":"50000"},{"midPx":"3000"}]`))
			} else {
				_, _ = w.Write([]byte(`{}`))
			}
		default: // /exchange
			_, _ = w.Write([]byte(`{"status":"ok","response":{"type":"order","data":{"statuses":[{"resting":{"oid":42}}]}}}`))
		}
	}))
	defer srv.Close()

	seed := make([]byte, 32)
	adapter, err := newHyperliquid(Credentials{
		ExchangeType: "hyperliquid",
		WalletAddr:   "0x1234",
		PrivateKey:   hex.EncodeToString(seed),
		BaseURL:      srv.URL,
	})
	if err != nil {
		t.Fatalf("newHyperliquid: %v", err)
	}
	res, err := adapter.OpenLong(t.Context(), OrderParams{Symbol: "BTC-USDT", Quantity: 0.01, Leverage: 5})
	if err != nil {
		t.Fatalf("OpenLong: %v", err)
	}
	if res.ExchangeOrderID != "42" {
		t.Errorf("order id: %s", res.ExchangeOrderID)
	}
	for _, want := range []string{`"type":"order"`, `"nonce"`, `"signature"`, `"r":"`, `"s":"`, `"vaultAddress":null`, `"a":0`, `"b":true`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("body missing %s: %s", want, gotBody)
		}
	}
}

// TestHLBalance 余额解析（clearinghouseState）。
func TestHLBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"crossMarginSummary":{"accountValue":"10050.5","withdrawable":"9800"}}`))
	}))
	defer srv.Close()
	adapter, _ := newHyperliquid(Credentials{ExchangeType: "hyperliquid", WalletAddr: "0x1", BaseURL: srv.URL})
	bal, err := adapter.GetBalance(t.Context())
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal != 10050.5 {
		t.Errorf("balance: %v", bal)
	}
}

// TestHLFills 成交解析（userFills：side B/A、closedPnl）。
func TestHLFills(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"coin":"BTC","px":"50000","sz":"0.001","side":"B","time":1754000000000,"closedPnl":"0","oid":1,"tid":99,"fee":"0.05","feeToken":"USDC"}]`))
	}))
	defer srv.Close()
	adapter, _ := newHyperliquid(Credentials{ExchangeType: "hyperliquid", WalletAddr: "0x1", BaseURL: srv.URL})
	fills, err := adapter.GetFills(t.Context(), 0)
	if err != nil {
		t.Fatalf("GetFills: %v", err)
	}
	if len(fills) != 1 || fills[0].ExchangeTradeID != "99" || fills[0].Side != "buy" || fills[0].CommissionAsset != "USDC" {
		t.Errorf("fills: %+v", fills)
	}
	if fills[0].Symbol != "BTC-USDT" {
		t.Errorf("symbol normalization: %s", fills[0].Symbol)
	}
}

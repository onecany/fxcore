package exchange

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newQueryServer 构造返回指定 body 的 mock server（按 path 分发）。
func newQueryServer(t *testing.T, paths map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		for p, body := range paths {
			if strings.Contains(r.URL.Path, p) {
				_, _ = w.Write([]byte(body))
				return
			}
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestBybitGetPositions 持仓查询解析（size 转数量、过滤 0 持仓）。
func TestBybitGetPositions(t *testing.T) {
	srv := newQueryServer(t, map[string]string{
		"position/list": `{"retCode":0,"result":{"list":[
			{"symbol":"BTCUSDT","side":"Buy","size":"0.01","avgPrice":"60000","markPrice":"60500","unrealisedPnl":"5.00","leverage":"5"},
			{"symbol":"ETHUSDT","side":"Buy","size":"0","avgPrice":"3000","markPrice":"3010","unrealisedPnl":"0","leverage":"5"}
		]}}`,
	})
	adapter, err := newBybit(Credentials{ExchangeType: "bybit", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newBybit: %v", err)
	}
	positions, err := adapter.GetPositions(t.Context())
	if err != nil {
		t.Fatalf("GetPositions: %v", err)
	}
	// 0 数量持仓被过滤 → 只留 1 条
	if len(positions) != 1 {
		t.Fatalf("want 1 position (zero-size filtered), got %d: %+v", len(positions), positions)
	}
	p := positions[0]
	// 实现契约：Side 统一归一化为小写（long/short）
	if p.Symbol != "BTCUSDT" || p.Side != "long" || p.Quantity != 0.01 || p.EntryPrice != 60000 {
		t.Fatalf("position parsed wrong: %+v", p)
	}
	if p.MarkPrice != 60500 || p.UnrealizedPnL != 5.00 || p.Leverage != 5 {
		t.Fatalf("position fields wrong: %+v", p)
	}
}

// TestBybitGetOpenOrders 挂单查询解析。
func TestBybitGetOpenOrders(t *testing.T) {
	srv := newQueryServer(t, map[string]string{
		"order/realtime": `{"retCode":0,"result":{"list":[
			{"orderId":"bybit-o1","symbol":"BTCUSDT","side":"Sell","qty":"0.01","price":"61000","orderStatus":"New","orderLinkId":"cid-1"}
		]}}`,
	})
	adapter, err := newBybit(Credentials{ExchangeType: "bybit", APIKey: "k", SecretKey: "s", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newBybit: %v", err)
	}
	orders, err := adapter.GetOpenOrders(t.Context())
	if err != nil {
		t.Fatalf("GetOpenOrders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("want 1 order, got %d: %+v", len(orders), orders)
	}
	o := orders[0]
	// 实现契约：Side 统一归一化为小写（buy/sell）
	if o.ExchangeOrderID != "bybit-o1" || o.Symbol != "BTCUSDT" || o.Side != "sell" || o.Quantity != 0.01 || o.Price != 61000 {
		t.Fatalf("order parsed wrong: %+v", o)
	}
}

// TestOKXGetPositions 持仓查询解析（OKX 结构：pos 数组）。
func TestOKXGetPositions(t *testing.T) {
	srv := newQueryServer(t, map[string]string{
		"account/positions": `{"code":"0","msg":"","data":[
			{"instId":"BTC-USDT-SWAP","posSide":"long","pos":"1","avgPx":"60000","markPx":"60500","upl":"5.00","lever":"5","mgnMode":"isolated"}
		]}`,
	})
	adapter, err := newOKX(Credentials{ExchangeType: "okx", APIKey: "k", SecretKey: "s", Passphrase: "p", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newOKX: %v", err)
	}
	positions, err := adapter.GetPositions(t.Context())
	if err != nil {
		t.Fatalf("GetPositions: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("want 1 position, got %d: %+v", len(positions), positions)
	}
	p := positions[0]
	if p.Symbol != "BTC-USDT-SWAP" || p.Side != "long" || p.Quantity != 1 || p.EntryPrice != 60000 || p.Leverage != 5 {
		t.Fatalf("position parsed wrong: %+v", p)
	}
}

// TestOKXGetOpenOrders 挂单查询解析。
func TestOKXGetOpenOrders(t *testing.T) {
	srv := newQueryServer(t, map[string]string{
		"trade/orders-pending": `{"code":"0","msg":"","data":[
			{"ordId":"okx-o1","instId":"BTC-USDT-SWAP","side":"buy","sz":"0.01","px":"61000","state":"live","clOrdId":"cid-1"}
		]}`,
	})
	adapter, err := newOKX(Credentials{ExchangeType: "okx", APIKey: "k", SecretKey: "s", Passphrase: "p", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newOKX: %v", err)
	}
	orders, err := adapter.GetOpenOrders(t.Context())
	if err != nil {
		t.Fatalf("GetOpenOrders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("want 1 order, got %d: %+v", len(orders), orders)
	}
	o := orders[0]
	if o.ExchangeOrderID != "okx-o1" || o.Symbol != "BTC-USDT-SWAP" || o.Side != "buy" || o.Quantity != 0.01 || o.Price != 61000 {
		t.Fatalf("order parsed wrong: %+v", o)
	}
}

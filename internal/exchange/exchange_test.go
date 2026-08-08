package exchange

import (
	"testing"
)

func TestFormats(t *testing.T) {
	cases := []struct {
		name string
		got  float64
		want float64
	}{
		{"binance floor to 0.01", FormatBinance(1.234, 0.01), 1.23},
		{"binance floor to 0.1", FormatBinance(1.25, 0.1), 1.2},
		{"bybit same as binance", FormatBybit(0.015, 0.01), 0.01},
		{"okx integer contracts", FormatOKX(3.7), 3},
		{"hyperliquid 5 sig digits", FormatHyperliquid(0.123456), 0.12346},
		{"hyperliquid large", FormatHyperliquid(1234.567), 1234.6},
		{"gate floor", FormatGate(9.9), 9},
		{"bitget 4 decimals", FormatBitget(1.23456, 4), 1.2345},
		{"kucoin round to step", FormatKuCoin(1.236, 0.05), 1.25},
		{"aster round tick", FormatAster(1.234, 0.1), 1.2},
		{"indodax floor", FormatIndodax(2.99), 2},
		{"lighter 4 decimals", FormatLighter(1.23456), 1.2345},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestFormatQtyDispatch(t *testing.T) {
	if v, err := formatQty("binance", 1.234, 0.01); err != nil || v != 1.23 {
		t.Errorf("binance dispatch: %v %v", v, err)
	}
	if v, err := formatQty("hyperliquid", 0.123456, 0); err != nil || v != 0.12346 {
		t.Errorf("hyperliquid dispatch: %v %v", v, err)
	}
	if _, err := formatQty("nope", 1, 0); err == nil {
		t.Errorf("expected error for unknown exchange")
	}
}

func TestMockAdapterLifecycle(t *testing.T) {
	m := NewMock(10000)
	ctx := t.Context()
	// 开多
	res, err := m.OpenLong(ctx, OrderParams{Symbol: "BTC-USDT", Quantity: 10, Price: 100})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if res.Status != "FILLED" {
		t.Errorf("status: %s", res.Status)
	}
	// 重复开仓拒绝（同方向）
	if _, err := m.OpenLong(ctx, OrderParams{Symbol: "BTC-USDT", Quantity: 5, Price: 100}); err == nil {
		t.Errorf("expected error on duplicate position")
	}
	// 余额占用
	bal, _ := m.GetBalance(ctx)
	if bal != 9000 {
		t.Errorf("balance: %v", bal)
	}
	// 平多：价格 110 → pnl = 10*10 = 100；保证金 1000 返还 → 9000+1000+100=10100
	_, err = m.CloseLong(ctx, OrderParams{Symbol: "BTC-USDT", Quantity: 10, Price: 110})
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	bal, _ = m.GetBalance(ctx)
	if bal != 10100 {
		t.Errorf("balance after close: %v want 10100", bal)
	}
	// 成交记录
	fills, _ := m.GetFills(ctx, 0)
	if len(fills) != 2 {
		t.Errorf("fills: %d", len(fills))
	}
}

func TestMockInsufficient(t *testing.T) {
	m := NewMock(100)
	ctx := t.Context()
	if _, err := m.OpenLong(ctx, OrderParams{Symbol: "BTC-USDT", Quantity: 10, Price: 100}); err == nil {
		t.Errorf("expected insufficient balance error")
	}
}

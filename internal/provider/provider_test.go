package provider

import (
	"encoding/json"
	"testing"

	"fxcore/internal/api/v1/dto"
)

// rawF 构造 json.RawMessage 数值。
func rawF(f float64) json.RawMessage {
	b, _ := json.Marshal(f)
	return b
}

func TestParseOKXRow(t *testing.T) {
	row := []string{"1754000000000", "50000.5", "50100", "49900", "50050.25", "123.4", "6170000", "6170000", "0"}
	k, err := parseOKXRow(row)
	if err != nil {
		t.Fatalf("parseOKXRow: %v", err)
	}
	want := dto.KlineDTO{Timestamp: 1754000000, Open: 50000.5, High: 50100, Low: 49900, Close: 50050.25, Volume: 123.4}
	if k != want {
		t.Errorf("got %+v want %+v", k, want)
	}
}

func TestParseHLCandle(t *testing.T) {
	// hl candle: [ts,o,h,l,c,v]，ts 为毫秒
	c := hlCandle{
		rawF(1754000000000), rawF(50000.5), rawF(50100), rawF(49900), rawF(50050.25), rawF(123.4),
	}
	k, err := parseHLCandle(c)
	if err != nil {
		t.Fatalf("parseHLCandle: %v", err)
	}
	if k.Timestamp != 1754000000 || k.Open != 50000.5 || k.Close != 50050.25 {
		t.Errorf("unexpected: %+v", k)
	}
}

func TestIntervalConversions(t *testing.T) {
	if hlInterval("1H") != "1h" || hlInterval("4D") != "15m" {
		t.Errorf("hlInterval mapping wrong")
	}
	if okxBar("1h") != "1H" || okxBar("4h") != "4H" || okxBar("15m") != "15m" {
		t.Errorf("okxBar mapping wrong")
	}
	if intervalMillis("15m") != 900_000 || intervalMillis("1d") != 86_400_000 {
		t.Errorf("intervalMillis wrong")
	}
}

func TestClampLimit(t *testing.T) {
	if clampLimit(0) != 200 || clampLimit(5000) != 1000 || clampLimit(50) != 50 {
		t.Errorf("clampLimit wrong")
	}
}

func TestNormalizeSymbol(t *testing.T) {
	if normalizeSymbol(" btc-usdt ") != "BTC-USDT" {
		t.Errorf("normalizeSymbol wrong")
	}
}

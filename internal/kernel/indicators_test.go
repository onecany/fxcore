package kernel

import (
	"math"
	"testing"

	"fxcore/internal/api/v1/dto"
)

// 已知序列验证：EMA(3) 手工计算。
func TestEMASeries(t *testing.T) {
	closes := []float64{1, 2, 3, 4, 5}
	// 种子 = 前 3 根 SMA = 2；α = 2/4 = 0.5
	// EMA3 = 2, 0.5*4+0.5*2=3, 0.5*5+0.5*3=4
	got := emaSeries(closes, 3)
	want := []float64{2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("emaSeries len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("ema[%d]=%.6f want %.6f", i, got[i], want[i])
		}
	}
}

// EMA 数据不足时返回 nil。
func TestEMANotEnoughData(t *testing.T) {
	if got := emaSeries([]float64{1, 2}, 3); got != nil {
		t.Fatalf("want nil for insufficient data, got %v", got)
	}
}

// RSI(14) 全涨序列 → 100；全跌序列 → 0。
func TestRSIAllUpAllDown(t *testing.T) {
	up := make([]float64, 30)
	down := make([]float64, 30)
	for i := 0; i < 30; i++ {
		up[i] = float64(i + 1)         // 严格递增
		down[i] = float64(100 - i)     // 严格递减
	}
	if v, ok := rsiLast(up, 14); !ok || v != 100 {
		t.Fatalf("all-up RSI=%v (ok=%v), want 100", v, ok)
	}
	if v, ok := rsiLast(down, 14); !ok || v != 0 {
		t.Fatalf("all-down RSI=%v (ok=%v), want 0", v, ok)
	}
}

// MACD 数据不足（<26 根）返回 ok=false。
func TestMACDNotEnoughData(t *testing.T) {
	short := make([]float64, 20)
	for i := range short {
		short[i] = float64(i + 1)
	}
	if _, _, _, ok := macd(short); ok {
		t.Fatalf("want ok=false for <26 candles")
	}
}

// MACD 足够数据时返回有限值。
func TestMACDSufficient(t *testing.T) {
	closes := make([]float64, 60)
	for i := range closes {
		closes[i] = 100 + float64(i)*0.5
	}
	dif, dea, hist, ok := macd(closes)
	if !ok {
		t.Fatalf("want ok=true for 60 candles")
	}
	if math.IsNaN(dif) || math.IsNaN(dea) || math.IsNaN(hist) {
		t.Fatalf("NaN in MACD: dif=%v dea=%v hist=%v", dif, dea, hist)
	}
	if dif < 0 {
		t.Fatalf("uptrend dif should be positive, got %f", dif)
	}
}

// BuildKlineContext：按 config 开关输出 K 线与指标。
func TestBuildKlineContext(t *testing.T) {
	klines := make([]dto.KlineDTO, 0, 40)
	for i := 0; i < 40; i++ {
		klines = append(klines, dto.KlineDTO{
			Timestamp: int64(1700000000 + i*60),
			Open:      100, High: 101, Low: 99, Close: 100 + float64(i)*0.1, Volume: 1000,
		})
	}
	cfg := dto.StrategyConfig{
		Indicators: dto.IndicatorConfig{
			EnableEMA:  true,
			EMAPeriods: []int{5},
			EnableMACD: true,
			EnableRSI:  true,
			RSIPeriods: []int{14},
			EnableBoll: true,
			BollPeriods: []int{20},
		},
	}
	out := BuildKlineContext(klines, cfg)
	if out == "" {
		t.Fatalf("expected non-empty context")
	}
	for _, want := range []string{"Market data", "t=1700001200", "EMA5=", "MACD:", "RSI14=", "BOLL20:"} {
		if !containsStr(out, want) {
			t.Fatalf("missing %q in context:\n%s", want, out)
		}
	}
}

// BOLL 手工值：period=4，序列 1..5 → 窗口 [2,3,4,5] 中轨 3.5。
func TestBOLLManual(t *testing.T) {
	closes := []float64{1, 2, 3, 4, 5}
	mid, upper, lower, ok := boll(closes, 4)
	if !ok {
		t.Fatalf("want ok=true")
	}
	if math.Abs(mid-3.5) > 1e-9 {
		t.Fatalf("mid=%.6f want 3.5", mid)
	}
	// 窗口 [2,3,4,5]：均 3.5，方差 = ((1.5²+0.5²+0.5²+1.5²)/4) = 5/4 = 1.25，std=√1.25≈1.118
	wantStd := math.Sqrt(1.25)
	if math.Abs(upper-(3.5+2*wantStd)) > 1e-9 {
		t.Fatalf("upper=%.6f want %.6f", upper, 3.5+2*wantStd)
	}
	if math.Abs(lower-(3.5-2*wantStd)) > 1e-9 {
		t.Fatalf("lower=%.6f want %.6f", lower, 3.5-2*wantStd)
	}
}

// BOLL 数据不足（<period 根）→ ok=false。
func TestBOLLNotEnoughData(t *testing.T) {
	if _, _, _, ok := boll([]float64{1, 2, 3}, 4); ok {
		t.Fatalf("want ok=false for insufficient data")
	}
}

// BuildKlineContext：60+ 根时省略中间蜡烛。
func TestBuildKlineContextOmitMiddle(t *testing.T) {
	klines := make([]dto.KlineDTO, 0, 80)
	for i := 0; i < 80; i++ {
		klines = append(klines, dto.KlineDTO{
			Timestamp: int64(1700000000 + i*60),
			Open: 100, High: 101, Low: 99, Close: 100, Volume: 1000,
		})
	}
	out := BuildKlineContext(klines, dto.StrategyConfig{})
	if !containsStr(out, "earlier candles omitted") {
		t.Fatalf("expected omit marker for 80 candles:\n%s", out)
	}
}

// BuildKlineContext：空 K 线返回空串。
func TestBuildKlineContextEmpty(t *testing.T) {
	if out := BuildKlineContext(nil, dto.StrategyConfig{}); out != "" {
		t.Fatalf("expected empty for no klines, got %q", out)
	}
}

// BuildKlineSummary 摘要格式。
func TestBuildKlineSummary(t *testing.T) {
	klines := []dto.KlineDTO{
		{Timestamp: 1, Open: 100, High: 105, Low: 98, Close: 100},
		{Timestamp: 2, Open: 100, High: 106, Low: 99, Close: 104},
	}
	out := BuildKlineSummary(klines)
	if !containsStr(out, "2 candles") || !containsStr(out, "100.0000") {
		t.Fatalf("unexpected summary: %q", out)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// ATR 手工值：period=2。
// TR[1] = max(14-11, |14-10|, |11-10|) = max(3,4,1) = 4；TR[2] = max(15-12, |15-12|, |12-12|) = 3
// 种子 SMA = (4+3)/2 = 3.5。
func TestATRManual(t *testing.T) {
	highs := []float64{11, 14, 15}
	lows := []float64{9, 11, 12}
	closes := []float64{10, 12, 13}
	v, ok := atrLast(highs, lows, closes, 2)
	if !ok {
		t.Fatalf("want ok=true")
	}
	if math.Abs(v-3.5) > 1e-9 {
		t.Fatalf("ATR=%.6f want 3.5", v)
	}
}

// ATR 数据不足（len <= period）→ ok=false。
func TestATRNotEnoughData(t *testing.T) {
	if _, ok := atrLast([]float64{1}, []float64{1}, []float64{1}, 2); ok {
		t.Fatalf("want ok=false for insufficient data")
	}
}

// volumeStats：最近一根 vs 前 window 根均量。
func TestVolumeStats(t *testing.T) {
	volumes := make([]float64, 0, 22)
	for i := 0; i < 21; i++ {
		volumes = append(volumes, 1000)
	}
	volumes = append(volumes, 2000)
	last, avg, ratio, ok := volumeStats(volumes, 20)
	if !ok || last != 2000 || avg != 1000 || math.Abs(ratio-2.0) > 1e-9 {
		t.Fatalf("volume stats = %v %v %v (ok=%v), want 2000/1000/2.0", last, avg, ratio, ok)
	}
}

// BuildKlineContext：ATR/Volume/OI/Funding 按 config 开关输出。
func TestBuildKlineContextExtIndicators(t *testing.T) {
	klines := make([]dto.KlineDTO, 0, 40)
	for i := 0; i < 40; i++ {
		o, h, l, c := 100.0, 102.0, 98.0, 101.0
		klines = append(klines, dto.KlineDTO{
			Timestamp: int64(1700000000 + i*60),
			Open:      o, High: h, Low: l, Close: c, Volume: 1000,
			OI: 500.5, Funding: 0.0001,
		})
	}
	cfg := dto.StrategyConfig{
		Indicators: dto.IndicatorConfig{
			EnableATR:         true,
			ATRPeriods:        []int{14},
			EnableVolume:      true,
			EnableOI:          true,
			EnableFundingRate: true,
		},
	}
	out := BuildKlineContext(klines, cfg)
	for _, want := range []string{"ATR14=", "Volume: last=", "Open Interest: 500.5000", "Funding rate: 0.000100"} {
		if !containsStr(out, want) {
			t.Fatalf("missing %q in context:\n%s", want, out)
		}
	}
}

// BuildKlineContextTF：多时间框架标签输出。
func TestBuildKlineContextTF(t *testing.T) {
	klines := []dto.KlineDTO{{Timestamp: 1, Open: 100, High: 101, Low: 99, Close: 100, Volume: 1000}}
	out := BuildKlineContextTF(klines, dto.StrategyConfig{}, "1h")
	if !containsStr(out, "Market data [1h]") {
		t.Fatalf("expected TF label in context:\n%s", out)
	}
}

package kernel

import (
	"fmt"
	"math"
	"strings"

	"fxcore/internal/api/v1/dto"
)

// ========== 技术指标计算（纯函数，供 BuildKlineContext 使用） ==========
// 对齐常见定义：EMA(α=2/(N+1))、MACD(12/26/9)、RSI(Wilder 平滑 14)。
// 输入 K 线按时间升序（provider 已归一化），不足周期数时返回 nil（跳过该指标）。

// emaSeries 计算 EMA 序列（α = 2/(period+1)，首个值 = 前 period 根 SMA 种子）。
func emaSeries(closes []float64, period int) []float64 {
	if len(closes) < period {
		return nil
	}
	out := make([]float64, 0, len(closes)-period+1)
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += closes[i]
	}
	prev := sum / float64(period)
	out = append(out, prev)
	alpha := 2.0 / float64(period+1)
	for i := period; i < len(closes); i++ {
		prev = alpha*closes[i] + (1-alpha)*prev
		out = append(out, prev)
	}
	return out
}

// emaLast 取 EMA 序列末值。
func emaLast(closes []float64, period int) (float64, bool) {
	series := emaSeries(closes, period)
	if len(series) == 0 {
		return 0, false
	}
	return series[len(series)-1], true
}

// macd 计算 MACD：快线 12 - 慢线 26 = DIF，DIF 的 9 期 EMA = DEA，柱 = (DIF-DEA)*2。
// 返回 (dif, dea, hist)，数据不足返回 ok=false。
func macd(closes []float64) (dif, dea, hist float64, ok bool) {
	fast, okF := emaLast(closes, 12)
	slow, okS := emaLast(closes, 26)
	if !okF || !okS {
		return 0, 0, 0, false
	}
	dif = fast - slow
	// DEA = DIF 的 9 期 EMA——需要 DIF 序列；简化：对收盘价序列的 DIF 近似
	// 精确做法：逐点算 DIF 序列再 EMA9。数据量小（200 根内）直接算全序列。
	difSeries := make([]float64, 0, len(closes))
	for i := 25; i < len(closes); i++ {
		f, _ := emaLast(closes[:i+1], 12)
		s, _ := emaLast(closes[:i+1], 26)
		difSeries = append(difSeries, f-s)
	}
	if len(difSeries) < 9 {
		return 0, 0, 0, false
	}
	deaSeries := emaSeries(difSeries, 9)
	if len(deaSeries) == 0 {
		return 0, 0, 0, false
	}
	dea = deaSeries[len(deaSeries)-1]
	hist = (dif - dea) * 2
	return dif, dea, hist, true
}

// rsiSeries 计算 RSI（Wilder 平滑，14 默认）：先算涨跌幅，平均增益/损失用 Wilder 递推。
func rsiSeries(closes []float64, period int) []float64 {
	if len(closes) <= period {
		return nil
	}
	gains := make([]float64, 0, len(closes)-period)
	losses := make([]float64, 0, len(closes)-period)
	for i := 1; i < len(closes); i++ {
		chg := closes[i] - closes[i-1]
		if chg >= 0 {
			gains = append(gains, chg)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, -chg)
		}
	}
	// Wilder 种子：前 period 个变化的简单平均
	avgGain, avgLoss := 0.0, 0.0
	for i := 0; i < period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)
	out := make([]float64, 0, len(gains)-period+1)
	out = append(out, rsiValue(avgGain, avgLoss))
	for i := period; i < len(gains); i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
		out = append(out, rsiValue(avgGain, avgLoss))
	}
	return out
}

func rsiValue(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}

// rsiLast 取 RSI 序列末值。
func rsiLast(closes []float64, period int) (float64, bool) {
	series := rsiSeries(closes, period)
	if len(series) == 0 {
		return 0, false
	}
	return series[len(series)-1], true
}

// boll 计算布林带：中轨 = period 期 SMA，上下轨 = 中轨 ± k×标准差（k=2 默认）。
// 返回 (mid, upper, lower)，数据不足返回 ok=false。
func boll(closes []float64, period int) (mid, upper, lower float64, ok bool) {
	if len(closes) < period {
		return 0, 0, 0, false
	}
	window := closes[len(closes)-period:]
	sum := 0.0
	for _, c := range window {
		sum += c
	}
	mid = sum / float64(period)
	variance := 0.0
	for _, c := range window {
		d := c - mid
		variance += d * d
	}
	std := 0.0
	if period > 1 {
		std = math.Sqrt(variance / float64(period))
	}
	upper = mid + 2*std
	lower = mid - 2*std
	return mid, upper, lower, true
}

// ========== K 线 + 指标 → 用户上下文 ==========

// BuildKlineContext 把 K 线数据与技术指标计算值格式化为 user 消息内容。
// 输出：最近 N 根 OHLCV 摘要 + 按 config 开关启用的指标值（EMA 周期列表 / MACD / RSI 周期列表）。
// klines 按时间升序（provider 归一化）；指标数据不足时自动跳过，不产生错误。
func BuildKlineContext(klines []dto.KlineDTO, cfg dto.StrategyConfig) string {
	if len(klines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Market data (candles, newest last):\n")
	// 全量输出最多 60 根（防 token 爆炸；summary 行覆盖窗口外）
	from := 0
	if len(klines) > 60 {
		from = len(klines) - 60
		b.WriteString(fmt.Sprintf("[... %d earlier candles omitted]\n", from))
	}
	for i := from; i < len(klines); i++ {
		k := klines[i]
		fmt.Fprintf(&b, "t=%d o=%.4f h=%.4f l=%.4f c=%.4f v=%.1f\n",
			k.Timestamp, k.Open, k.High, k.Low, k.Close, k.Volume)
	}
	// 技术指标（按 config 开关）
	closes := make([]float64, 0, len(klines))
	for _, k := range klines {
		closes = append(closes, k.Close)
	}
	ind := cfg.Indicators
	if ind.EnableEMA && len(ind.EMAPeriods) > 0 {
		vals := make([]string, 0, len(ind.EMAPeriods))
		for _, p := range ind.EMAPeriods {
			if v, ok := emaLast(closes, p); ok {
				vals = append(vals, fmt.Sprintf("EMA%d=%.4f", p, v))
			}
		}
		if len(vals) > 0 {
			b.WriteString("Indicators: " + strings.Join(vals, " ") + "\n")
		}
	}
	if ind.EnableMACD {
		if dif, dea, hist, ok := macd(closes); ok {
			b.WriteString(fmt.Sprintf("MACD: dif=%.4f dea=%.4f hist=%.4f\n", dif, dea, hist))
		}
	}
	if ind.EnableRSI && len(ind.RSIPeriods) > 0 {
		vals := make([]string, 0, len(ind.RSIPeriods))
		for _, p := range ind.RSIPeriods {
			if v, ok := rsiLast(closes, p); ok {
				vals = append(vals, fmt.Sprintf("RSI%d=%.1f", p, v))
			}
		}
		if len(vals) > 0 {
			b.WriteString("Indicators: " + strings.Join(vals, " ") + "\n")
		}
	}
	if ind.EnableBoll && len(ind.BollPeriods) > 0 {
		for _, p := range ind.BollPeriods {
			if mid, upper, lower, ok := boll(closes, p); ok {
				b.WriteString(fmt.Sprintf("BOLL%d: mid=%.4f upper=%.4f lower=%.4f\n", p, mid, upper, lower))
			}
		}
	}
	return b.String()
}

// BuildKlineSummary 轻量摘要（user 消息开头）：窗口外数据的一句话统计。
func BuildKlineSummary(klines []dto.KlineDTO) string {
	if len(klines) == 0 {
		return ""
	}
	first, last := klines[0], klines[len(klines)-1]
	return fmt.Sprintf("%d candles %.4f→%.4f (high %.4f low %.4f)",
		len(klines), first.Close, last.Close, last.High, last.Low)
}

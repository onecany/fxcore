package exchange

import (
	"fmt"
	"math"
)

// 数量格式策略（API设计.md §14.2 契约，纯函数可测）：
//   binance LOT_SIZE   floor(qty/step)*step
//   bybit              floor(qty/qtyStep)*qtyStep（step 缓存）
//   okx                合约张数（整数）
//   hyperliquid        5 位有效数字
//   gate               取整
//   bitget             VolumePlace（小数位）
//   kucoin             round 到 step
//   aster              roundToTickSize
//   indodax            floor
//   lighter            %.4f

// FormatBinance binance LOT_SIZE：floor 到 step 的整数倍（收尾到 1e-8 消除浮点尾差）。
func FormatBinance(qty, step float64) float64 {
	if step <= 0 {
		step = 0.01
	}
	return snap(math.Floor(qty/step) * step)
}

// FormatBybit bybit：floor 到 qtyStep 整数倍。
func FormatBybit(qty, step float64) float64 {
	return FormatBinance(qty, step)
}

// FormatOKX okx：合约张数（整数）。
func FormatOKX(qty float64) float64 {
	return math.Floor(qty)
}

// FormatHyperliquid hyperliquid：5 位有效数字。
func FormatHyperliquid(qty float64) float64 {
	if qty <= 0 {
		return 0
	}
	// 5 位有效数字：先取数量级，再 round
	digits := math.Ceil(math.Log10(qty))
	mult := math.Pow(10, 5-digits)
	return math.Round(qty*mult) / mult
}

// FormatGate gate：取整。
func FormatGate(qty float64) float64 {
	return math.Floor(qty)
}

// FormatBitget bitget：VolumePlace 小数位。
func FormatBitget(qty float64, place int) float64 {
	mult := math.Pow(10, float64(place))
	return math.Floor(qty*mult) / mult
}

// FormatKuCoin kucoin：round 到 step 整数倍。
func FormatKuCoin(qty, step float64) float64 {
	if step <= 0 {
		step = 0.01
	}
	return snap(math.Round(qty/step) * step)
}

// FormatAster aster：roundToTickSize。
func FormatAster(qty, tickSize float64) float64 {
	return FormatKuCoin(qty, tickSize)
}

// snap 收尾到 1e-8（消除浮点乘法尾差，如 1.2000000000000002 → 1.2）。
func snap(v float64) float64 {
	return math.Round(v*1e8) / 1e8
}

// FormatIndodax indodax：floor。
func FormatIndodax(qty float64) float64 {
	return math.Floor(qty)
}

// FormatLighter lighter：%.4f。
func FormatLighter(qty float64) float64 {
	return math.Floor(qty*10000) / 10000
}

// formatQty 按交易所类型格式化数量（引擎层统一入口；step 由适配器查询填充）。
func formatQty(exType string, qty, step float64) (float64, error) {
	switch exType {
	case "binance":
		return FormatBinance(qty, step), nil
	case "bybit":
		return FormatBybit(qty, step), nil
	case "okx":
		return FormatOKX(qty), nil
	case "hyperliquid":
		return FormatHyperliquid(qty), nil
	case "gate":
		return FormatGate(qty), nil
	case "bitget":
		return FormatBitget(qty, 4), nil
	case "kucoin":
		return FormatKuCoin(qty, step), nil
	case "aster":
		return FormatAster(qty, step), nil
	case "indodax":
		return FormatIndodax(qty), nil
	case "lighter":
		return FormatLighter(qty), nil
	default:
		return 0, fmt.Errorf("%w: unknown exchange %q", ErrParams, exType)
	}
}

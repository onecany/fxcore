package engine

import (
	"math"
	"strings"

	"fxcore/internal/exchange"
	"fxcore/internal/model"
)

// 风控实现（API设计.md §14.1 强制公式）：
//   max_positions=3；BTC/ETH 仓值 ≤5×equity、alt ≤1×equity；
//   最小仓位 12 USDT（BTC/ETH ≥60）；保证金 1.01/lev+0.001；
//   超限按 98% 缩减；回撤监控（盈利>5% 且自峰回撤≥40% 强平）。

// MarginRate 保证金率公式：1.01/lev + 0.001（§14.1）。
func MarginRate(leverage int) float64 {
	if leverage <= 0 {
		leverage = 1
	}
	return 1.01/float64(leverage) + 0.001
}

// isBTCEth 主流币判定（BTC/ETH 及其变体）。
func isBTCEth(symbol string) bool {
	up := strings.ToUpper(symbol)
	return strings.HasPrefix(up, "BTC") || strings.HasPrefix(up, "ETH")
}

// minPositionSize 最小仓位（§16：BTC/ETH≥60、其余≥12 USDT）。
func minPositionSize(symbol string) float64 {
	if isBTCEth(symbol) {
		return 60
	}
	return 12
}

// filterByRisk 对 AI 决策做风控过滤（§14.1）：
// 开仓动作逐条检查：持仓数上限、仓值比、最小仓位、保证金占用；
// 超限开仓动作丢弃（记录到错误，不中断周期）。
func (e *Engine) filterByRisk(t *model.Trader, adapter exchange.Adapter, balance float64, actions []model.DecisionAction) []model.DecisionAction {
	cfg := e.strategyConfig(t)
	rc := cfg.RiskControl
	if rc.MaxPositions <= 0 {
		rc.MaxPositions = 3
	}

	openCount := len(e.store.ListPositionsByTrader(t.ID))
	out := make([]model.DecisionAction, 0, len(actions))
	for _, a := range actions {
		switch a.Action {
		case model.ActionCloseLong, model.ActionCloseShort, model.ActionHold, model.ActionWait:
			out = append(out, a)
			continue
		case model.ActionOpenLong, model.ActionOpenShort:
			// 1. 持仓数上限
			if openCount >= rc.MaxPositions {
				e.noteTraderError(t.ID, "risk: max_positions reached, drop "+a.Action+" "+a.Symbol)
				continue
			}
			// 2. 最小仓位（按名义价值估算）
			minUsd := minPositionSize(a.Symbol)
			if a.Quantity > 0 && a.Price > 0 {
				notional := a.Quantity * a.Price
				if notional < minUsd {
					e.noteTraderError(t.ID, "risk: position too small, drop "+a.Symbol)
					continue
				}
			}
			// 3. 仓值比（BTC/ETH ≤5×equity，alt ≤1×equity）
			if balance > 0 && a.Quantity > 0 && a.Price > 0 {
				ratio := 1.0
				if isBTCEth(a.Symbol) {
					ratio = rc.BTCEthMaxPositionValueRatio
				} else {
					ratio = rc.AltcoinMaxPositionValueRatio
				}
				if ratio <= 0 {
					ratio = 1
				}
				notional := computePositionValue(a.Quantity, a.Price)
				if notional > ratio*balance {
					e.noteTraderError(t.ID, "risk: position value exceeds ratio, drop "+a.Symbol)
					continue
				}
				// 4. 保证金占用 ≤ max_margin_usage
				if rc.MaxMarginUsage > 0 {
					margin := marginNeeded(notional, int(a.Leverage))
					if margin > rc.MaxMarginUsage*balance {
						e.noteTraderError(t.ID, "risk: margin usage exceeds limit, drop "+a.Symbol)
						continue
					}
				}
			}
			// 5. 杠杆上限
			lev := int(a.Leverage)
			if lev <= 0 {
				lev = 1
			}
			maxLev := rc.AltcoinMaxLeverage
			if isBTCEth(a.Symbol) {
				maxLev = rc.BTCEthMaxLeverage
			}
			if maxLev > 0 && lev > maxLev {
				// §16：杠杆超限自动降档（非报错）
				a.Leverage = float64(maxLev)
			}
			openCount++
			out = append(out, a)
		}
	}
	return out
}

// maxDrawdownGuard 回撤守卫：盈利>5% 且自峰回撤 ≥40% 时强平全部持仓（§14.1）。
// 返回需要强平的 symbol 列表。
func (e *Engine) maxDrawdownGuard(traderID string, positions []exchange.Position, equity float64) []string {
	// 峰值从最近权益快照序列计算
	snapshots := e.store.ListEquityByTrader(traderID)
	if len(snapshots) == 0 {
		return nil
	}
	peak := snapshots[0].TotalEquity
	for _, s := range snapshots {
		if s.TotalEquity > peak {
			peak = s.TotalEquity
		}
	}
	// 盈利>5%（相对首个快照）且回撤 ≥40%
	base := snapshots[0].TotalEquity
	if base <= 0 || peak <= base*1.05 {
		return nil
	}
	if peak <= 0 {
		return nil
	}
	if (peak-equity)/peak < 0.40 {
		return nil
	}
	// 触发：返回全部持仓 symbol
	out := make([]string, 0, len(positions))
	for _, p := range positions {
		out = append(out, p.Symbol)
	}
	return out
}

// computePositionValue 名义价值（数量×价格，无价格时按 0 处理）。
func computePositionValue(qty, price float64) float64 {
	if qty <= 0 || price <= 0 || math.IsNaN(qty*price) || math.IsInf(qty*price, 0) {
		return 0
	}
	return qty * price
}

// marginNeeded 按杠杆计算所需保证金（1.01/lev+0.001 × 名义价值）。
func marginNeeded(notional float64, leverage int) float64 {
	return notional * MarginRate(leverage)
}

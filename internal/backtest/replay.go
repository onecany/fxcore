package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/kernel"
	"fxcore/internal/llm"
	"fxcore/internal/model"
)

// runLoop 回放主循环：每 cadence 推进一个窗口周期。
// 顺序（对齐 §14.1 语义）：取 K 线 → 构建上下文 → AI 决策 → 成交执行 → 权益快照。
func (e *Engine) runLoop(ctx context.Context, runID string, r *run) {
	cfg := r.cfg
	windowStart := cfg.StartTime
	lastSaved := time.Time{}

	ticker := time.NewTicker(2 * time.Second) // 每 tick 推进一个周期（回放加速：2s=1周期）
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.syncState(runID)
			return
		case <-ticker.C:
		}

		r.mu.Lock()
		if r.state == dto.BacktestPaused {
			r.mu.Unlock()
			continue
		}
		if r.state != dto.BacktestRunning {
			r.mu.Unlock()
			return
		}
		r.cycle++
		cycle := r.cycle
		r.mu.Unlock()

		// 窗口推进
		windowEnd := windowStart + int64(cfg.Cadence*60)
		if windowStart >= cfg.EndTime {
			e.finish(runID, r, dto.BacktestCompleted, "")
			return
		}

		// 逐 symbol 决策执行
		for _, symbol := range cfg.Symbols {
			if !e.stillRunning(r) {
				return
			}
			klines, err := e.klines.Klines(ctx, symbol, "15m", 200)
			if err != nil {
				e.noteError(r, fmt.Sprintf("klines %s: %v", symbol, err))
				continue // 单 symbol 失败不中断周期（§14.1）
			}
			e.decideAndExecute(ctx, runID, r, symbol, klines, cycle)
		}

		// 权益快照（先于任何 UI 展示）
		e.snapshot(runID, r, cycle, windowEnd)

		// 回撤监控：盈利>5% 且自峰回撤 ≥40% 强平（§14.1）
		if r.peakEquity > cfg.InitialBalance*1.05 && r.maxDrawdown() >= 40 {
			e.liquidateAll(runID, r, cycle, "drawdown_guard")
			e.finish(runID, r, dto.BacktestLiquidated, "drawdown guard: peak >5% profit, drawdown >=40%")
			return
		}
		if r.equity <= 0 {
			e.finish(runID, r, dto.BacktestLiquidated, "equity depleted")
			return
		}

		windowStart = windowEnd

		// 进度持久化节流：每 10 周期或 5s
		now := time.Now()
		if cycle%10 == 0 || now.Sub(lastSaved) > 5*time.Second {
			e.syncState(runID)
			lastSaved = now
		}
	}
}

// stillRunning 检查状态（paused 时等待，stopped/failed 时退出）。
func (e *Engine) stillRunning(r *run) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state == dto.BacktestRunning
}

// decideAndExecute 单 symbol：取模型 → 构建提示词 → AI 决策 → 执行。
func (e *Engine) decideAndExecute(ctx context.Context, runID string, r *run, symbol string, klines []dto.KlineDTO, cycle int64) {
	var systemMsg string

	// 模型来源：cfg.StrategyID 指向的策略无模型字段（内存模型为 backtest 专属）——
	// 本实现用 dto.BacktestConfig 里显式 model 配置的降级：优先第一个可用模型。
	if m := e.pickModel(); m != nil {
		systemMsg = buildReplayPrompt(r, symbol, klines)
		res, err := e.ai.Chat(ctx, m, llm.ChatRequest{
			Messages: []llm.Message{
				{Role: "system", Content: systemMsg},
				{Role: "user", Content: "Analyze the current market and output your decision."},
			},
			Temperature: 0.3,
			MaxTokens:   2048,
		})
		rec := &model.DecisionRecord{
			TraderID:    runID,
			CycleNumber: cycle,
			Timestamp:   time.Now().UTC(),
			SystemPrompt: systemMsg,
			Success:     false,
		}
		var actions []model.DecisionAction
		if err != nil {
			rec.ErrorMessage = err.Error()
			rec.AIRequestDurationMS = resLatency(res, err)
			actions = kernel.SafeWait()
		} else {
			rec.RawResponse = res.Content
			rec.AIRequestDurationMS = res.LatencyMS
			if parsed, perr := kernel.ParseDecision(res.Content); perr != nil {
				rec.ErrorMessage = perr.Error()
				actions = kernel.SafeWait()
			} else {
				rec.Success = true
				actions = parsed
			}
		}
		rec.Decisions = actions
		execLog, _ := json.Marshal(actions)
		rec.ExecutionLog = execLog
		e.saveDecision(runID, cycle, symbol, rec)

		// 执行（close 优先于 open，§14.1）
		e.executeActions(ctx, runID, r, symbol, klines, actions, cycle)
	}
}

// pickModel 取一个可用模型（回测默认用第一个已存模型）。
func (e *Engine) pickModel() *llm.Model {
	for _, m := range e.store.ListModels("") {
		if m.DeletedAt != nil || m.Status == "error" {
			continue
		}
		if modelCfg, ok := e.models.GetModel(m.ID); ok {
			return modelCfg
		}
	}
	return nil
}

// executeActions 逐条执行（失败不中断周期，成功间隔可略——回放无真实下单）。
func (e *Engine) executeActions(ctx context.Context, runID string, r *run, symbol string, klines []dto.KlineDTO, actions []model.DecisionAction, cycle int64) {
	for _, a := range actions {
		if a.Symbol == "" {
			a.Symbol = symbol
		}
		if a.Symbol != symbol {
			continue // 只处理当前 symbol
		}
		r.mu.Lock()
		pos := r.positions[symbol]
		r.mu.Unlock()

		switch a.Action {
		case model.ActionOpenLong, model.ActionOpenShort:
			if pos != nil {
				continue // 同方向已有持仓不加仓（§12 1403 语义）
			}
			e.openPosition(runID, r, symbol, a, klines, cycle)
		case model.ActionCloseLong, model.ActionCloseShort:
			if pos == nil || !sideMatches(pos.Side, a.Action) {
				continue
			}
			e.closePosition(runID, r, symbol, a, klines, cycle)
		case model.ActionWait, model.ActionHold:
			continue
		}
	}
}

// sideMatches close 动作与持仓方向匹配。
func sideMatches(posSide, action string) bool {
	if posSide == "long" {
		return action == model.ActionCloseLong
	}
	return action == model.ActionCloseShort
}

// openPosition 开仓：按 fill_policy 定价，扣手续费与滑点。
func (e *Engine) openPosition(runID string, r *run, symbol string, a model.DecisionAction, klines []dto.KlineDTO, cycle int64) {
	if a.Quantity <= 0 {
		a.Quantity = 0.001 // 默认最小仓位
	}
	price := fillPrice(a, klines, dto.FillPolicy(r.cfg.FillPolicy), true)
	if price <= 0 {
		return
	}
	notional := a.Quantity * price
	r.mu.Lock()
	defer r.mu.Unlock()
	if notional > r.available {
		return // 余额不足不开仓
	}
	fee := notional * r.cfg.FeeBPS / 10000
	slippage := notional * r.cfg.SlippageBPS / 10000
	r.available -= notional + fee
	r.equity -= fee + slippage
	side := "long"
	if a.Action == model.ActionOpenShort {
		side = "short"
	}
	r.positions[symbol] = &btPosition{Symbol: symbol, Side: side, Qty: a.Quantity, Entry: price}
	e.store.AddBacktestTrade(&model.BacktestTrade{
		RunID: runID, Timestamp: time.Now().Unix(), Symbol: symbol,
		Action: a.Action, Side: side, Quantity: a.Quantity, Price: price,
		Fee: fee, Slippage: slippage, OrderValue: notional, Leverage: int(a.Leverage), Cycle: cycle,
		PositionAfter: a.Quantity, Note: "open",
	})
}

// closePosition 平仓：实现盈亏 = (现价-开仓价)*数量*方向；保证金返还。
func (e *Engine) closePosition(runID string, r *run, symbol string, a model.DecisionAction, klines []dto.KlineDTO, cycle int64) {
	price := fillPrice(a, klines, dto.FillPolicy(r.cfg.FillPolicy), false)
	if price <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	pos, ok := r.positions[symbol]
	if !ok {
		return
	}
	dir := 1.0
	if pos.Side == "short" {
		dir = -1.0
	}
	realized := (price - pos.Entry) * pos.Qty * dir
	fee := pos.Qty * price * r.cfg.FeeBPS / 10000
	slippage := pos.Qty * price * r.cfg.SlippageBPS / 10000
	// 保证金返还（名义/杠杆）后结算盈亏
	margin := pos.Qty * pos.Entry / float64(r.cfg.Leverage)
	r.available += margin + realized
	r.equity += realized - fee - slippage
	delete(r.positions, symbol)
	r.peakEquity = maxF(r.peakEquity, r.equity)
	e.store.AddBacktestTrade(&model.BacktestTrade{
		RunID: runID, Timestamp: time.Now().Unix(), Symbol: symbol,
		Action: a.Action, Side: pos.Side, Quantity: pos.Qty, Price: price,
		Fee: fee, Slippage: slippage, RealizedPnL: realized, Leverage: r.cfg.Leverage,
		Cycle: cycle, PositionAfter: 0, Note: "close",
	})
}

// liquidateAll 强平全部持仓（回撤守卫/权益耗尽）。
func (e *Engine) liquidateAll(runID string, r *run, cycle int64, reason string) {
	r.mu.Lock()
	positions := make(map[string]*btPosition, len(r.positions))
	for k, v := range r.positions {
		positions[k] = v
	}
	r.mu.Unlock()
	for symbol, pos := range positions {
		dir := 1.0
		if pos.Side == "short" {
			dir = -1.0
		}
		realized := (pos.Entry*0.95 - pos.Entry) * pos.Qty * dir // 近似 5% 强平价
		r.mu.Lock()
		r.equity += realized
		delete(r.positions, symbol)
		r.mu.Unlock()
		e.store.AddBacktestTrade(&model.BacktestTrade{
			RunID: runID, Timestamp: time.Now().Unix(), Symbol: symbol,
			Action: "liquidate", Side: pos.Side, Quantity: pos.Qty, Price: pos.Entry * 0.95,
			RealizedPnL: realized, Cycle: cycle, Liquidation: true, Note: reason,
		})
	}
	r.mu.Lock()
	r.liquidated = true
	r.mu.Unlock()
}

// snapshot 权益快照（含浮盈亏与回撤）。
func (e *Engine) snapshot(runID string, r *run, cycle int64, ts int64) {
	r.mu.Lock()
	equity := r.equity
	available := r.available
	peak := r.peakEquity
	unrealized := 0.0
	for _, pos := range r.positions {
		unrealized += pos.Qty * (pos.Entry*1.01 - pos.Entry) // 近似浮盈亏（1% 变动）
	}
	total := equity + unrealized
	if total > peak {
		r.peakEquity = total
	}
	dd := 0.0
	if peak > 0 && peak > total {
		dd = (peak - total) / peak * 100
	}
	r.mu.Unlock()

	e.store.AddBacktestEquity(&model.BacktestEquity{
		RunID: runID, Timestamp: ts, Equity: total, Available: available,
		PnL: total - r.cfg.InitialBalance,
		PnLPct: (total - r.cfg.InitialBalance) / r.cfg.InitialBalance * 100,
		DrawdownPct: dd, Cycle: cycle,
	})
}

// finish 终态。
func (e *Engine) finish(runID string, r *run, state, errMsg string) {
	r.mu.Lock()
	r.state = state
	if errMsg != "" {
		r.lastError = errMsg
	}
	r.cancel()
	r.mu.Unlock()
	e.syncState(runID)
}

// noteError 记录非致命错误。
func (e *Engine) noteError(r *run, msg string) {
	r.mu.Lock()
	r.lastError = msg
	r.mu.Unlock()
}

// saveDecision 持久化决策（按 run+cycle，多 symbol 合并到 payload JSON 数组）。
func (e *Engine) saveDecision(runID string, cycle int64, symbol string, rec *model.DecisionRecord) {
	payload, _ := json.Marshal(rec)
	// 追加语义：同 cycle 多 symbol 分别落一条（cycle 相同、symbol 不同）
	e.store.AddBacktestDecision(&model.BacktestDecision{
		RunID:   runID,
		Cycle:   cycle,
		Payload: payload,
	})
}

// fillPrice 按 fill_policy 定价（§10 FillPolicy）。
func fillPrice(a model.DecisionAction, klines []dto.KlineDTO, policy dto.FillPolicy, opening bool) float64 {
	if len(klines) == 0 {
		return 0
	}
	last := klines[len(klines)-1]
	switch policy {
	case dto.FillNextOpen:
		// 下一根开盘价 ≈ 当前收盘（简化：取倒数第二根收盘作为下根开盘近似）
		if opening && len(klines) > 1 {
			return klines[len(klines)-2].Close
		}
		return last.Close
	case dto.FillBarVWAP:
		return (last.High + last.Low + last.Close) / 3
	default: // mid
		return (last.High + last.Low) / 2
	}
}

// buildReplayPrompt 回放提示词（基础版：K 线摘要 + 仓位 + 输出契约）。
func buildReplayPrompt(r *run, symbol string, klines []dto.KlineDTO) string {
	if len(klines) == 0 {
		return "No market data available."
	}
	var b strings.Builder
	b.WriteString("You are an experienced crypto futures trader running a backtest.\n")
	fmt.Fprintf(&b, "Symbol: %s | Initial balance: %.2f USDT | Leverage: %d\n", symbol, r.cfg.InitialBalance, r.cfg.Leverage)
	start := len(klines) - 20
	if start < 0 {
		start = 0
	}
	fmt.Fprintf(&b, "Recent %d bars (OHLCV, oldest first):\n", len(klines)-start)
	for _, k := range klines[start:] {
		fmt.Fprintf(&b, "ts=%d o=%.4f h=%.4f l=%.4f c=%.4f v=%.0f\n", k.Timestamp, k.Open, k.High, k.Low, k.Close, k.Volume)
	}
	r.mu.Lock()
	pos := r.positions[symbol]
	r.mu.Unlock()
	if pos != nil {
		fmt.Fprintf(&b, "Current position: %s %s qty=%.6f entry=%.4f\n", pos.Side, symbol, pos.Qty, pos.Entry)
	} else {
		b.WriteString("Current position: none\n")
	}
	b.WriteString(`Output format (STRICT):
<reasoning>brief analysis</reasoning>
<decision>[{"action":"open_long|open_short|close_long|close_short|hold|wait","symbol":"` + symbol + `","quantity":0.001,"leverage":5,"confidence":80,"stop_loss":0.95,"take_profit":1.05}]</decision>
Only output the reasoning and decision tags.`)
	return b.String()
}

func resLatency(res *llm.ChatResult, err error) int64 {
	if res != nil {
		return res.LatencyMS
	}
	return 0
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// Package engine 交易引擎（API设计.md §14.1 行为契约）。
// 每交易员独立 goroutine：首周期立即执行，之后按 scan_interval ticker；
// 状态由 store 驱动（paused 等待 / stopped 退出）；风控见 risk.go；
// 持仓持久化走 OrderSync + PositionBuilder（ordersync.go）。
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/exchange"
	"fxcore/internal/kernel"
	"fxcore/internal/llm"
	"fxcore/internal/model"
	"fxcore/internal/provider"
	"fxcore/internal/store"
)

// CredentialsResolver 交易所凭据解析（组装层注入，引擎不感知 RSA 解密）。
type CredentialsResolver interface {
	// Resolve 按交易所类型 + 属主用户取可用账户凭据（已解密，多用户隔离）。
	Resolve(exchangeType, userID string) (*exchange.Credentials, bool)
}

// Engine 交易引擎管理器。
type Engine struct {
	store  *store.Store
	creds  CredentialsResolver
	models llm.ModelProvider
	ai     *llm.Client
	klines provider.KlineProvider

	mu      sync.Mutex
	runners map[string]*runner // traderID → 运行控制
}

// runner 单交易员运行控制。
type runner struct {
	traderID string
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewEngine 构造交易引擎。
func NewEngine(s *store.Store, creds CredentialsResolver, models llm.ModelProvider, ai *llm.Client, klines provider.KlineProvider) *Engine {
	return &Engine{
		store:   s,
		creds:   creds,
		models:  models,
		ai:      ai,
		klines:  klines,
		runners: make(map[string]*runner),
	}
}

// IsRunning 交易员是否已挂引擎。
func (e *Engine) IsRunning(traderID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.runners[traderID]
	return ok
}

// Start 启动交易员（起主循环 + OrderSync）。
func (e *Engine) Start(traderID string) error {
	e.mu.Lock()
	if _, exists := e.runners[traderID]; exists {
		e.mu.Unlock()
		return fmt.Errorf("engine: trader already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &runner{traderID: traderID, cancel: cancel}
	e.runners[traderID] = r
	e.mu.Unlock()

	r.wg.Add(2)
	go func() { defer r.wg.Done(); e.loop(ctx, traderID) }()
	go func() { defer r.wg.Done(); e.orderSyncLoop(ctx, traderID) }()
	return nil
}

// Stop 停止交易员（等待循环退出）。
func (e *Engine) Stop(traderID string) {
	e.mu.Lock()
	r, ok := e.runners[traderID]
	if ok {
		delete(e.runners, traderID)
	}
	e.mu.Unlock()
	if !ok {
		return
	}
	r.cancel()
	r.wg.Wait()
}

// ========== 主循环 ==========

// loop 交易循环：首周期立即执行，之后按 interval ticker。
// 状态机由 store 驱动：paused 等待、stopped 退出（§14.1）。
func (e *Engine) loop(ctx context.Context, traderID string) {
	// 任何退出路径（GetTrader 失败 / stopped / ctx 取消）都清理 runner，
	// 避免 trader 被删后 IsRunning 永久 true 导致无法重建（P2-7）。
	defer func() {
		e.mu.Lock()
		delete(e.runners, traderID)
		e.mu.Unlock()
	}()
	first := true
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		t, ok := e.store.GetTrader(traderID)
		if !ok {
			return
		}
		switch t.Status {
		case model.StatusStopped, model.StatusError:
			return // 停止即退出
		case model.StatusPaused:
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		case model.StatusRunning:
			e.runCycle(ctx, t)
		}

		interval := 60
		if t.Schedule != nil && t.Schedule.Interval > 0 {
			interval = t.Schedule.Interval
		}
		if interval < 3 {
			interval = 3 // 最小 3 秒（§14.1 最小 3 分钟语义的秒级实现）
		}
		if first {
			first = false
		} else {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(interval) * time.Second):
			}
		}
	}
}

// runCycle 单周期（§14.1 顺序）：
// 每日 PnL 重置 → 构建上下文（余额→持仓→K线→统计）→ 权益快照 →
// AI 决策(balanced) → close 优先 → 逐条执行（失败不中断，成功间隔 1s）。
func (e *Engine) runCycle(ctx context.Context, t *model.Trader) {
	// 每日 PnL 重置（24h 窗口）
	e.resetDailyPnL(t.ID)

	adapter, err := e.adapterFor(t)
	if err != nil {
		e.noteTraderError(t.ID, err.Error())
		return
	}

	// 上下文：余额 + 持仓
	balance, err := adapter.GetBalance(ctx)
	if err != nil {
		e.noteTraderError(t.ID, err.Error())
		return
	}
	positions, err := adapter.GetPositions(ctx)
	if err != nil {
		e.noteTraderError(t.ID, err.Error())
		return
	}

	// 权益快照（与 AI 无关，先存）
	e.snapshotEquity(t.ID, balance, len(positions))

	// 回撤守卫：盈利>5% 且自峰回撤 ≥40% 强平（§14.1）
	if victims := e.maxDrawdownGuard(t.ID, positions, balance); len(victims) > 0 {
		for _, sym := range victims {
			for _, p := range positions {
				if p.Symbol != sym {
					continue
				}
				var res *exchange.OrderResult
				if p.Side == "long" {
					res, err = adapter.CloseLong(ctx, exchange.OrderParams{Symbol: sym, Quantity: p.Quantity, ReduceOnly: true})
				} else {
					res, err = adapter.CloseShort(ctx, exchange.OrderParams{Symbol: sym, Quantity: p.Quantity, ReduceOnly: true})
				}
				if err != nil {
					e.noteTraderError(t.ID, "drawdown guard close failed: "+err.Error())
					continue
				}
				e.closePosition(t.ID, sym, res.AvgFillPrice)
			}
		}
		e.noteTraderError(t.ID, "risk: drawdown guard triggered (peak profit >5%, drawdown >=40%)")
		return // 本轮不再开新仓
	}

	// 候选币：策略 coin_source 的静态币（缺省 BTC/ETH）
	symbols := e.candidateSymbols(t)
	if len(symbols) == 0 {
		return
	}

	// AI 决策
	actions := e.decide(ctx, t, adapter, balance, positions, symbols)
	if len(actions) == 0 {
		return
	}

	// close 优先于 open（§14.1）
	closeFirst := make([]model.DecisionAction, 0, len(actions))
	openActions := make([]model.DecisionAction, 0, len(actions))
	for _, a := range actions {
		switch a.Action {
		case model.ActionCloseLong, model.ActionCloseShort:
			closeFirst = append(closeFirst, a)
		default:
			openActions = append(openActions, a)
		}
	}
	ordered := append(closeFirst, openActions...)

	// 逐条执行：失败不中断周期；成功间隔 1s
	for i, a := range ordered {
		if ctx.Err() != nil {
			return
		}
		e.execute(ctx, t, adapter, a)
		if i < len(ordered)-1 {
			time.Sleep(time.Second)
		}
	}
}

// decide 构建提示词 → AI 调用 → 解析 → 风控过滤。
func (e *Engine) decide(ctx context.Context, t *model.Trader, adapter exchange.Adapter, balance float64, positions []exchange.Position, symbols []string) []model.DecisionAction {
	m, ok := e.models.GetModel(t.ModelConfig.ModelID)
	if !ok {
		e.noteTraderError(t.ID, "model not found: "+t.ModelConfig.ModelID)
		return nil
	}
	cfg := e.strategyConfig(t)
	system := kernel.BuildSystemPrompt(cfg)

	posLines := make([]string, 0, len(positions))
	for _, p := range positions {
		posLines = append(posLines, fmt.Sprintf("%s %s qty=%.4f entry=%.4f", p.Symbol, p.Side, p.Quantity, p.EntryPrice))
	}
	var userContext string
	if e.klines != nil && len(symbols) > 0 {
		// 按 config 的周期/数量拉取真实 K 线，计算指标后作为 user 上下文（2026-08 修复：
		// 原实现只取 20 根最后一根收盘价摘要，EMA/MACD/RSI 开关与周期从未生效）
		kc := cfg.Indicators.Klines
		primary := kc.PrimaryTimeframe
		if primary == "" {
			primary = "15m"
		}
		count := kc.PrimaryCount
		if count <= 0 {
			count = 200
		}
		// 时间框架列表：primary + 多时间框架开关启用的 selected_timeframes（去重）
		tfs := []string{primary}
		seen := map[string]bool{primary: true}
		if kc.EnableMultiTimeframe {
			for _, tf := range kc.SelectedTimeframes {
				if tf == "" || seen[tf] {
					continue
				}
				seen[tf] = true
				tfs = append(tfs, tf)
			}
		}
		var market strings.Builder
		for _, tf := range tfs {
			ks, err := e.klines.Klines(ctx, symbols[0], tf, count)
			if err != nil || len(ks) == 0 {
				// 拉取失败不中断周期，但必须可观测（2026-08：原实现静默吞错，
				// 导致 USER PROMPT 无 Market data 且日志零痕迹）
				e.noteTraderError(t.ID, fmt.Sprintf("klines %s %s failed: %v", symbols[0], tf, err))
				continue
			}
			if len(tfs) > 1 {
				market.WriteString(kernel.BuildKlineContextTF(ks, cfg, tf))
			} else {
				market.WriteString(kernel.BuildKlineContext(ks, cfg))
			}
		}
		if market.Len() > 0 {
			userContext = kernel.BuildUserContext(balance, posLines, market.String())
		}
	}
	if userContext == "" {
		userContext = kernel.BuildUserContext(balance, posLines, "")
	}

	rec := &model.DecisionRecord{
		TraderID:     t.ID,
		Timestamp:    time.Now().UTC(),
		SystemPrompt: system,
		InputPrompt:  userContext,
	}
	res, err := e.ai.Chat(ctx, m, llm.ChatRequest{
		Messages:    []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: userContext}},
		Temperature: 0.3,
		MaxTokens:   2048,
	})
	if err != nil {
		rec.ErrorMessage = err.Error()
		rec.Success = false
		rec.Decisions = kernel.SafeWait()
		rec.ExecutionLog = mustJSON(rec.Decisions)
		e.store.AddDecision(rec)
		e.noteTraderError(t.ID, "ai decision failed: "+err.Error())
		return nil
	}
	rec.RawResponse = res.Content
	rec.AIRequestDurationMS = res.LatencyMS
	actions, perr := kernel.ParseDecision(res.Content)
	if perr != nil {
		rec.Success = false
		rec.ErrorMessage = perr.Error()
		rec.Decisions = kernel.SafeWait()
	} else {
		rec.Success = true
		rec.Decisions = actions
	}
	rec.ExecutionLog = mustJSON(rec.Decisions)
	e.store.AddDecision(rec)

	// 风控过滤（§14.1）
	return e.filterByRisk(t, adapter, balance, actions)
}

// execute 单条执行（幂等 client_order_id + 持仓持久化）。
func (e *Engine) execute(ctx context.Context, t *model.Trader, adapter exchange.Adapter, a model.DecisionAction) {
	// 幂等键：traderID+cycle+symbol+action（§14.1）
	cid := fmt.Sprintf("%s-%d-%s-%s", t.ID, time.Now().Unix(), a.Symbol, a.Action)
	p := exchange.OrderParams{
		Symbol:        a.Symbol,
		Quantity:      a.Quantity,
		Leverage:      int(a.Leverage),
		ClientOrderID: cid,
	}
	var res *exchange.OrderResult
	var err error
	switch a.Action {
	case model.ActionOpenLong:
		res, err = adapter.OpenLong(ctx, p)
	case model.ActionOpenShort:
		res, err = adapter.OpenShort(ctx, p)
	case model.ActionCloseLong:
		p.ReduceOnly = true
		res, err = adapter.CloseLong(ctx, p)
	case model.ActionCloseShort:
		p.ReduceOnly = true
		res, err = adapter.CloseShort(ctx, p)
	default:
		return // hold/wait
	}
	if err != nil {
		e.noteTraderError(t.ID, fmt.Sprintf("execute %s: %v", a.Action, err))
		return
	}
	// 持仓持久化（引擎写 position 表；OrderSync 侧 PositionBuilder 做校正）
	if a.Action == model.ActionOpenLong || a.Action == model.ActionOpenShort {
		side := "long"
		if a.Action == model.ActionOpenShort {
			side = "short"
		}
		e.store.AddPosition(&model.Position{
			TraderID:   t.ID,
			Symbol:     a.Symbol,
			Side:       side,
			Size:       a.Quantity,
			Quantity:   a.Quantity,
			EntryPrice: res.AvgFillPrice,
			MarkPrice:  res.AvgFillPrice,
			Leverage:   int(a.Leverage),
			Status:     model.PositionOpen,
			Source:     "engine",
		})
	}
	if a.Action == model.ActionCloseLong || a.Action == model.ActionCloseShort {
		e.closePosition(t.ID, a.Symbol, res.AvgFillPrice)
	}
	// 订单留痕
	e.store.AddOrder(&model.Order{
		TraderID:      t.ID,
		Symbol:        a.Symbol,
		Side:          mapSide(a.Action),
		PositionSide:  mapPositionSide(a.Action),
		Type:          "market",
		Quantity:      a.Quantity,
		Status:        res.Status,
		FilledQuantity: res.FilledQuantity,
		AvgFillPrice:  res.AvgFillPrice,
		ExchangeOrderID: res.ExchangeOrderID,
		ClientOrderID:  cid,
		Leverage:      int(a.Leverage),
		ReduceOnly:    a.Action == model.ActionCloseLong || a.Action == model.ActionCloseShort,
	})
}

// closePosition 平仓：按持仓 entry 结算 realized pnl + 回写交易员指标。
func (e *Engine) closePosition(traderID, symbol string, exitPrice float64) {
	// 找到该交易员该 symbol 的 OPEN 持仓
	positions := e.store.ListPositionsByTrader(traderID)
	var target *model.Position
	for _, p := range positions {
		if p.Symbol == symbol {
			target = p
			break
		}
	}
	if target == nil {
		return
	}
	dir := 1.0
	if target.Side == "short" {
		dir = -1.0
	}
	pnl := (exitPrice - target.EntryPrice) * target.Quantity * dir
	e.store.ClosePosition(target.ID, pnl)
	e.store.WithTraderAndPositions(traderID, func(t *model.Trader, all []*model.Position) error {
		m := t.Metrics
		m.TradeCount++
		m.TotalPnL += pnl
		m.DailyPnL += pnl
		m.WinRate = calcWinRateLocal(all, traderID)
		t.Metrics = m
		return nil
	})
}

// adapterFor 构造适配器（凭据解析 + 工厂）。
func (e *Engine) adapterFor(t *model.Trader) (exchange.Adapter, error) {
	if e.creds == nil {
		return nil, fmt.Errorf("engine: no credentials resolver")
	}
	creds, ok := e.creds.Resolve(t.Exchange, t.UserID)
	if !ok {
		return nil, fmt.Errorf("engine: no exchange account for %s", t.Exchange)
	}
	return exchange.New(*creds)
}

// strategyConfig 交易员策略配置（缺省用默认）。
func (e *Engine) strategyConfig(t *model.Trader) dto.StrategyConfig {
	cfg := dto.StrategyConfig{
		StrategyType: "ai",
		Language:     "zh",
		CoinSource:   dto.CoinSourceConfig{SourceType: dto.CoinSourceStatic},
		Indicators: dto.IndicatorConfig{
			Klines: dto.KlineConfig{PrimaryTimeframe: "15m", PrimaryCount: 200},
		},
		RiskControl: dto.RiskControlConfig{
			MaxPositions: 3, BTCEthMaxLeverage: 5, AltcoinMaxLeverage: 5,
			BTCEthMaxPositionValueRatio: 5, AltcoinMaxPositionValueRatio: 1,
			MinPositionSize: 12, MinRiskRewardRatio: 1.5, MinConfidence: 0.6,
		},
	}
	if st, ok := e.store.GetStrategy(t.StrategyID); ok && len(st.Config) > 0 {
		var sc dto.StrategyConfig
		if err := jsonUnmarshal(st.Config, &sc); err == nil {
			if sc.RiskControl.MaxPositions > 0 {
				cfg.RiskControl = sc.RiskControl
			}
			if sc.Indicators.Klines.PrimaryTimeframe != "" {
				cfg.Indicators = sc.Indicators
			}
			if sc.CoinSource.SourceType != "" {
				cfg.CoinSource = sc.CoinSource
			}
			cfg.CustomPrompt = sc.CustomPrompt
			cfg.PromptSections = sc.PromptSections
		}
	}
	return cfg
}

// candidateSymbols 候选币：策略静态币，缺省 BTC-USDT。
func (e *Engine) candidateSymbols(t *model.Trader) []string {
	cfg := e.strategyConfig(t)
	if len(cfg.CoinSource.StaticCoins) > 0 {
		return cfg.CoinSource.StaticCoins
	}
	return []string{"BTC-USDT"}
}

// noteTraderError 记录交易员级错误（logger 重定向到文件+终端）。
func (e *Engine) noteTraderError(traderID, msg string) {
	log.Printf("[trader:%s] %s", traderID, msg)
}

// resetDailyPnL 24h 窗口重置每日盈亏。
func (e *Engine) resetDailyPnL(traderID string) {
	e.store.WithTrader(traderID, func(t *model.Trader) error {
		m := t.Metrics
		m.DailyPnL = 0
		t.Metrics = m
		return nil
	})
}

// snapshotEquity 权益快照。
func (e *Engine) snapshotEquity(traderID string, balance float64, posCount int) {
	e.store.AddEquitySnapshot(&model.EquitySnapshot{
		TraderID:      traderID,
		TotalEquity:   balance,
		Balance:       balance,
		PositionCount: posCount,
	})
}

// calcWinRateLocal 胜率（引擎内部版，语义同 service.calcWinRate）。
func calcWinRateLocal(positions []*model.Position, traderID string) float64 {
	var wins, total float64
	for _, p := range positions {
		if p.TraderID != traderID || p.ClosedAt == nil {
			continue
		}
		total++
		if p.PnL > 0 {
			wins++
		}
	}
	if total == 0 {
		return 0
	}
	return wins / total
}

// 辅助映射
func mapSide(action string) string {
	switch action {
	case model.ActionOpenLong, model.ActionCloseShort:
		return "buy"
	default:
		return "sell"
	}
}

func mapPositionSide(action string) string {
	switch action {
	case model.ActionOpenLong, model.ActionCloseLong:
		return "long"
	default:
		return "short"
	}
}

// mustJSON 序列化辅助（失败返回空数组 JSON）。
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("[]")
	}
	return json.RawMessage(b)
}

// jsonUnmarshal 容错反序列化。
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

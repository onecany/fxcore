// Package backtest 回放引擎（API设计.md §11/§14 行为契约）。
// 状态机 created→running→paused→running→completed|failed|liquidated；
// 单用户全局锁：任意时刻最多一个 running run（1411）。
// 回放循环：K 线数据源链取数 → 构建提示词 → AI 决策 → 解析 → 按 fill_policy 成交。
package backtest

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/llm"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/provider"
	"fxcore/internal/store"
)

// Engine 回测引擎。
type Engine struct {
	store  *store.Store
	klines provider.KlineProvider
	models llm.ModelProvider
	ai     *llm.Client

	mu   sync.Mutex
	runs map[string]*run // run_id → 运行控制（进程内）
}

// run 单个回测运行（运行时状态，元数据持久化在 store）。
type run struct {
	mu          sync.Mutex
	userID      string // 属主用户（P1-8：每用户运行锁）
	state       string
	cfg         dto.BacktestConfig
	cancel      context.CancelFunc
	cycle       int64
	totalCycles int64
	equity      float64
	available   float64
	peakEquity  float64
	positions   map[string]*btPosition // symbol → 仓位
	liquidated  bool
	lastError   string
}

// btPosition 回测仓位。
type btPosition struct {
	Symbol string
	Side   string // long | short
	Qty    float64
	Entry  float64
}

// NewEngine 构造引擎（依赖注入：数据源链 + 模型提供者 + AI 客户端）。
func NewEngine(s *store.Store, klines provider.KlineProvider, models llm.ModelProvider, ai *llm.Client) *Engine {
	return &Engine{
		store:  s,
		klines: klines,
		models: models,
		ai:     ai,
		runs:   make(map[string]*run),
	}
}

// newRunID 生成唯一 run_id（随机后缀，§11）。
func newRunID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "bt_" + hex.EncodeToString(b) + "_" + fmt.Sprintf("%d", time.Now().Unix())
}

// Start 启动回测。校验配置 → 检查全局运行锁 → 建 run → 起 goroutine。
func (e *Engine) Start(userID string, cfg dto.BacktestConfig) (*dto.RunMetadata, *middleware.APIError) {
	if len(cfg.Symbols) == 0 {
		return nil, middleware.BadRequest("symbols is required", map[string]string{"symbols": "at least one symbol"})
	}
	if cfg.InitialBalance <= 0 {
		cfg.InitialBalance = 10000
	}
	if cfg.Leverage <= 0 {
		cfg.Leverage = 5
	}
	if cfg.FeeBPS < 0 {
		cfg.FeeBPS = 5 // 默认 0.05%
	}
	if cfg.SlippageBPS < 0 {
		cfg.SlippageBPS = 1
	}
	if cfg.FillPolicy == "" {
		cfg.FillPolicy = dto.FillNextOpen
	}
	// 时间窗：缺省近 30 天；cadence 缺省 15 分钟
	if cfg.StartTime <= 0 || cfg.EndTime <= 0 {
		now := time.Now()
		cfg.EndTime = now.Unix()
		cfg.StartTime = now.AddDate(0, 0, -30).Unix()
	}
	if cfg.EndTime <= cfg.StartTime {
		return nil, middleware.BadRequest("end_time must be after start_time", nil)
	}
	if cfg.Cadence <= 0 {
		cfg.Cadence = 15
	}

	e.mu.Lock()
	// 每用户运行锁（1411）：同属主用户已有 running run 时拒绝（P1-8：不跨用户互斥）
	for _, r := range e.runs {
		if r.isRunning() && (userID == "" || r.userID == "" || r.userID == userID) {
			e.mu.Unlock()
			return nil, middleware.BacktestLock("another backtest run is in progress")
		}
	}
	runID := newRunID()
	r := &run{
		userID:      userID,
		state:       dto.BacktestRunning,
		cfg:         cfg,
		equity:      cfg.InitialBalance,
		available:   cfg.InitialBalance,
		peakEquity:  cfg.InitialBalance,
		positions:   make(map[string]*btPosition),
		totalCycles: ((cfg.EndTime - cfg.StartTime) / int64(cfg.Cadence*60)) + 1,
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	e.runs[runID] = r
	e.mu.Unlock()

	// 模型信息落元数据
	aiProvider, aiModel := "", ""
	if cfg.StrategyID != "" {
		if st, ok := e.store.GetStrategy(cfg.StrategyID); ok {
			aiProvider = st.Name
		}
	}
	_ = aiProvider

	meta := &model.BacktestRun{
		RunID:      runID,
		UserID:     userID,
		State:      dto.BacktestRunning,
		SymbolCount: len(cfg.Symbols),
		AIModel:    aiModel,
	}
	cfgJSON, _ := json.Marshal(cfg)
	meta.ConfigJSON = string(cfgJSON)
	e.store.CreateBacktestRun(meta)

	go e.runLoop(ctx, runID, r)
	return e.meta(runID), nil
}

// Pause 暂停。
func (e *Engine) Pause(runID string) (*dto.RunMetadata, *middleware.APIError) {
	r, apiErr := e.getRun(runID)
	if apiErr != nil {
		return nil, apiErr
	}
	r.mu.Lock()
	if r.state != dto.BacktestRunning {
		state := r.state
		r.mu.Unlock()
		return nil, middleware.BadRequest("run is not running (state="+state+")", nil)
	}
	r.state = dto.BacktestPaused
	r.mu.Unlock()
	e.syncState(runID)
	return e.meta(runID), nil
}

// Resume 恢复。
func (e *Engine) Resume(runID string) (*dto.RunMetadata, *middleware.APIError) {
	r, apiErr := e.getRun(runID)
	if apiErr != nil {
		return nil, apiErr
	}
	r.mu.Lock()
	if r.state != dto.BacktestPaused {
		state := r.state
		r.mu.Unlock()
		return nil, middleware.BadRequest("run is not paused (state="+state+")", nil)
	}
	r.state = dto.BacktestRunning
	r.mu.Unlock()
	e.syncState(runID)
	return e.meta(runID), nil
}

// Stop 停止（不可恢复）。
func (e *Engine) Stop(runID string) (*dto.RunMetadata, *middleware.APIError) {
	r, apiErr := e.getRun(runID)
	if apiErr != nil {
		return nil, apiErr
	}
	r.mu.Lock()
	if r.state == dto.BacktestCompleted || r.state == dto.BacktestStopped || r.state == dto.BacktestFailed {
		state := r.state
		r.mu.Unlock()
		return nil, middleware.BadRequest("run already in terminal state ("+state+")", nil)
	}
	r.state = dto.BacktestStopped
	r.cancel()
	r.mu.Unlock()
	e.syncState(runID)
	return e.meta(runID), nil
}

// Label 打标签。
func (e *Engine) Label(runID, label string) (*dto.RunMetadata, *middleware.APIError) {
	if _, apiErr := e.getRun(runID); apiErr != nil {
		return nil, apiErr
	}
	meta, ok := e.store.GetBacktestRun(runID, "")
	if !ok {
		return nil, middleware.BacktestNotFound("run not found")
	}
	meta.Label = label
	e.store.UpdateBacktestRun(meta)
	return e.meta(runID), nil
}

// Delete 删除（含关联数据）。
func (e *Engine) Delete(runID string) *middleware.APIError {
	r, apiErr := e.getRun(runID)
	if apiErr != nil {
		return apiErr
	}
	r.mu.Lock()
	// 活跃状态（running/paused）禁止删除（1411）；注意已持锁，不得再调 isRunning()
	if r.state == dto.BacktestRunning || r.state == dto.BacktestPaused {
		r.mu.Unlock()
		return middleware.BacktestLock("cannot delete an active run")
	}
	r.cancel()
	r.mu.Unlock()
	e.mu.Lock()
	delete(e.runs, runID)
	e.mu.Unlock()
	if !e.store.DeleteBacktestRun(runID) {
		return middleware.BacktestNotFound("run not found")
	}
	return nil
}

// Status 状态载荷。
func (e *Engine) Status(runID string) (*dto.BacktestStatusPayload, *middleware.APIError) {
	r, apiErr := e.getRun(runID)
	if apiErr != nil {
		return nil, apiErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	meta, _ := e.store.GetBacktestRun(runID, "")
	meta.State = r.state
	meta.ProgressPct = r.progress()
	meta.EquityLast = r.equity
	meta.Liquidated = r.liquidated
	meta.LastError = r.lastError
	if r.peakEquity > 0 && r.peakEquity > r.equity {
		meta.MaxDrawdownPct = (r.peakEquity - r.equity) / r.peakEquity * 100
	}
	payload := metaToDTO(meta)
	return &dto.BacktestStatusPayload{
		RunMetadata:  *payload,
		CurrentCycle: r.cycle,
		TotalCycles:  r.totalCycles,
	}, nil
}

// List 运行列表（state 过滤 + 分页）。
func (e *Engine) List(userID, state, search string, page, size int) ([]*model.BacktestRun, int) {
	all := e.store.ListBacktestRuns(userID)
	var filtered []*model.BacktestRun
	for _, r := range all {
		if state != "" && r.State != state {
			continue
		}
		if search != "" && !strings.Contains(r.RunID, search) && !strings.Contains(r.Label, search) {
			continue
		}
		filtered = append(filtered, r)
	}
	total := len(filtered)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	// 合并运行时状态（进度/equity 以引擎实时为准）
	for _, r := range filtered {
		if rn, ok := e.runs[r.RunID]; ok {
			rn.mu.Lock()
			r.State = rn.state
			r.ProgressPct = rn.progress()
			r.EquityLast = rn.equity
			rn.mu.Unlock()
		}
	}
	return filtered[start:end], total
}

// OwnsRun 归属校验：run 存在且属主用户（UserID 空视为历史数据放行）。
func (e *Engine) OwnsRun(runID, userID string) bool {
	if runID == "" {
		return true
	}
	meta, ok := e.store.GetBacktestRun(runID, "")
	if !ok {
		return false
	}
	return userID == "" || meta.UserID == "" || meta.UserID == userID
}

// getRun 取运行控制（不存在回 1412）。
func (e *Engine) getRun(runID string) (*run, *middleware.APIError) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.runs[runID]
	if !ok {
		if rec, exists := e.store.GetBacktestRun(runID, ""); exists {
			// store 有记录但进程内无控制（进程重启）——
			// 终态 run 从 DB 恢复真实状态（重启后历史回测的指标/详情仍可查）；
			// 运行中态引擎已不持有控制（goroutine 随进程消亡），映射 failed 防止对无控制 run 误操作。
			switch rec.State {
			case dto.BacktestCompleted, dto.BacktestStopped, dto.BacktestLiquidated, dto.BacktestFailed:
				var cfg dto.BacktestConfig
				_ = json.Unmarshal([]byte(rec.ConfigJSON), &cfg)
				return &run{state: rec.State, cfg: cfg, equity: rec.EquityLast, liquidated: rec.Liquidated, lastError: rec.LastError}, nil
			}
			return &run{state: dto.BacktestFailed, lastError: "engine restarted"}, nil
		}
		return nil, middleware.BacktestNotFound("run not found")
	}
	return r, nil
}

// meta 汇总 RunMetadata（store 元数据 + 运行时状态）。
func (e *Engine) meta(runID string) *dto.RunMetadata {
	meta, ok := e.store.GetBacktestRun(runID, "")
	if !ok {
		return &dto.RunMetadata{RunID: runID, State: dto.BacktestFailed}
	}
	if r, exists := e.runs[runID]; exists {
		r.mu.Lock()
		meta.State = r.state
		meta.ProgressPct = r.progress()
		meta.EquityLast = r.equity
		meta.MaxDrawdownPct = r.maxDrawdown()
		meta.Liquidated = r.liquidated
		meta.LastError = r.lastError
		r.mu.Unlock()
	}
	return metaToDTO(meta)
}

// syncState 把运行状态持久化到 store 元数据。
func (e *Engine) syncState(runID string) {
	meta, ok := e.store.GetBacktestRun(runID, "")
	if !ok {
		return
	}
	if r, exists := e.runs[runID]; exists {
		r.mu.Lock()
		meta.State = r.state
		meta.ProgressPct = r.progress()
		meta.EquityLast = r.equity
		meta.Liquidated = r.liquidated
		meta.LastError = r.lastError
		r.mu.Unlock()
	}
	e.store.UpdateBacktestRun(meta)
}

// metaToDTO 实体转 DTO。
func metaToDTO(m *model.BacktestRun) *dto.RunMetadata {
	return &dto.RunMetadata{
		RunID:          m.RunID,
		State:          m.State,
		Label:          m.Label,
		SymbolCount:    m.SymbolCount,
		ProgressPct:    m.ProgressPct,
		EquityLast:     m.EquityLast,
		MaxDrawdownPct: m.MaxDrawdownPct,
		Liquidated:     m.Liquidated,
		LastError:      m.LastError,
		CreatedAt:      m.CreatedAt,
	}
}

// (r *run) 状态辅助
func (r *run) isRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state == dto.BacktestRunning
}

func (r *run) progress() float64 {
	if r.totalCycles <= 0 {
		return 0
	}
	p := float64(r.cycle) / float64(r.totalCycles) * 100
	if p > 100 {
		p = 100
	}
	return p
}

func (r *run) maxDrawdown() float64 {
	if r.peakEquity <= 0 {
		return 0
	}
	if r.peakEquity <= r.equity {
		return 0
	}
	return (r.peakEquity - r.equity) / r.peakEquity * 100
}

// AIProvider 解析（models 接口取模型名——策略 id 映射留调用方）。
var _ = errors.New

// Klines 按 run 配置取某 symbol 的 K 线（handler /backtest/klines 用）。
func (e *Engine) Klines(runID, symbol, timeframe string) ([]dto.KlineDTO, *middleware.APIError) {
	if _, apiErr := e.getRun(runID); apiErr != nil {
		return nil, apiErr
	}
	if timeframe == "" {
		timeframe = "15m"
	}
	klines, err := e.klines.Klines(context.Background(), symbol, timeframe, 200)
	if err != nil {
		return nil, middleware.NewAPIError(middleware.CodeExchangeFailed, 400, "fetch klines failed: "+err.Error())
	}
	return klines, nil
}

// Equity 权益序列（时间升序；limit 钳制 1..5000）。
func (e *Engine) Equity(runID string, limit int) ([]*model.BacktestEquity, *middleware.APIError) {
	if _, apiErr := e.getRun(runID); apiErr != nil {
		return nil, apiErr
	}
	all := e.store.ListBacktestEquities(runID)
	if limit <= 0 || limit > 5000 {
		limit = 5000
	}
	if len(all) > limit {
		// 降采样：取末尾 limit 条（最新段）
		all = all[len(all)-limit:]
	}
	return all, nil
}

// Trades 成交序列。
func (e *Engine) Trades(runID string) ([]*model.BacktestTrade, *middleware.APIError) {
	if _, apiErr := e.getRun(runID); apiErr != nil {
		return nil, apiErr
	}
	return e.store.ListBacktestTrades(runID), nil
}

// Metrics 回测指标；未就绪（非 completed）返回 ready=false（handler 映射 202）。
func (e *Engine) Metrics(runID string) (*dto.BacktestMetrics, bool, *middleware.APIError) {
	r, apiErr := e.getRun(runID)
	if apiErr != nil {
		return nil, false, apiErr
	}
	r.mu.Lock()
	ready := r.state == dto.BacktestCompleted
	equityLast := r.equity
	liquidated := r.liquidated
	r.mu.Unlock()
	if !ready {
		return nil, false, nil
	}
	trades := e.store.ListBacktestTrades(runID)
	m := &dto.BacktestMetrics{TotalTrades: len(trades), FinalEquity: equityLast, Liquidated: liquidated}
	if len(trades) == 0 {
		return m, true, nil
	}
	initial := r.cfg.InitialBalance
	if initial > 0 {
		m.ReturnPct = (equityLast - initial) / initial * 100
	}
	var wins, losses int
	var grossWin, grossLoss, totalPnL float64
	for _, t := range trades {
		if t.RealizedPnL > 0 {
			wins++
			grossWin += t.RealizedPnL
		} else if t.RealizedPnL < 0 {
			losses++
			grossLoss += t.RealizedPnL
		}
		totalPnL += t.RealizedPnL
	}
	if len(trades) > 0 {
		m.WinRate = float64(wins) / float64(len(trades))
	}
	m.TotalPnL = totalPnL
	if losses > 0 && grossLoss != 0 {
		m.ProfitFactor = grossWin / (-grossLoss)
	}
	if wins > 0 {
		m.AvgWin = grossWin / float64(wins)
	}
	if losses > 0 {
		m.AvgLoss = grossLoss / float64(losses)
	}
	// 回撤：从权益序列算
	equities := e.store.ListBacktestEquities(runID)
	peak := 0.0
	var maxDD float64
	for _, eq := range equities {
		if eq.Equity > peak {
			peak = eq.Equity
		}
		if peak > 0 && peak > eq.Equity {
			dd := (peak - eq.Equity) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	m.MaxDrawdownPct = maxDD
	return m, true, nil
}

// Trace 单周期决策（含完整 prompt/输出）。
func (e *Engine) Trace(runID string, cycle int64) (*model.DecisionRecord, *middleware.APIError) {
	if _, apiErr := e.getRun(runID); apiErr != nil {
		return nil, apiErr
	}
	d, ok := e.store.GetBacktestDecision(runID, cycle)
	if !ok {
		return nil, middleware.BacktestNotFound("decision not found for cycle")
	}
	var rec model.DecisionRecord
	if err := json.Unmarshal(d.Payload, &rec); err != nil {
		return nil, middleware.Internal("corrupt decision payload")
	}
	return &rec, nil
}

// Decisions 决策分页（周期降序）。
func (e *Engine) Decisions(runID string, page, size int) ([]*model.DecisionRecord, int, *middleware.APIError) {
	if _, apiErr := e.getRun(runID); apiErr != nil {
		return nil, 0, apiErr
	}
	all := e.store.ListBacktestDecisions(runID)
	total := len(all)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	out := make([]*model.DecisionRecord, 0, end-start)
	for _, d := range all[start:end] {
		var rec model.DecisionRecord
		if err := json.Unmarshal(d.Payload, &rec); err != nil {
			continue
		}
		out = append(out, &rec)
	}
	return out, total, nil
}

// Export 打包 zip（决策 + 成交 + 权益 JSON）。
func (e *Engine) Export(runID string) ([]byte, *middleware.APIError) {
	if _, apiErr := e.getRun(runID); apiErr != nil {
		return nil, apiErr
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeJSONEntry := func(name string, v any) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		return err
	}
	if err := writeJSONEntry("run.json", e.meta(runID)); err != nil {
		return nil, middleware.Internal("export run failed")
	}
	if err := writeJSONEntry("trades.json", e.store.ListBacktestTrades(runID)); err != nil {
		return nil, middleware.Internal("export trades failed")
	}
	if err := writeJSONEntry("equities.json", e.store.ListBacktestEquities(runID)); err != nil {
		return nil, middleware.Internal("export equities failed")
	}
	if err := writeJSONEntry("decisions.json", e.store.ListBacktestDecisions(runID)); err != nil {
		return nil, middleware.Internal("export decisions failed")
	}
	if err := zw.Close(); err != nil {
		return nil, middleware.Internal("close zip failed")
	}
	return buf.Bytes(), nil
}

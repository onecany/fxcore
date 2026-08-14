package service

import (
	"errors"
	"math"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/store"
	"fxcore/internal/trader/engine"
)

// TraderService 交易员生命周期状态机（文档 §8 防错清单）：
//   idle→running→paused→running→stopped
//   · 非法跳转 -> 1004
//   · running 时重复 start -> 1204（幂等）
//   · stopped→running 允许重启（对文档线性序列的合理扩展，见 data/API设计.md 复盘）
// 引擎挂载：eng 非空时 Start/Stop 同步驱动交易引擎 goroutine。
type TraderService struct {
	store  *store.Store
	engine *engine.Engine
}

// NewTraderService 构造交易员服务（eng 可空：仅状态机，不跑交易循环）。
func NewTraderService(s *store.Store, eng *engine.Engine) *TraderService {
	return &TraderService{store: s, engine: eng}
}

// transitions 状态迁移表。
var transitions = map[string]map[string]bool{
	model.StatusIdle:    {model.StatusRunning: true},
	model.StatusRunning: {model.StatusPaused: true, model.StatusStopped: true},
	model.StatusPaused:  {model.StatusRunning: true, model.StatusStopped: true},
	model.StatusStopped: {model.StatusRunning: true}, // 重启
	model.StatusError:   {model.StatusStopped: true},
}

// validExchanges 支持的交易所（10 家，§14.2；未接入适配器的类型运行时返回 ErrNotImplemented）。
var validExchanges = map[string]bool{
	"binance": true, "hyperliquid": true, "aster": true, "bybit": true, "okx": true,
	"bitget": true, "gate": true, "kucoin": true, "indodax": true, "lighter": true,
}

// Create 创建交易员（状态 idle）。
func (svc *TraderService) Create(userID string, in *dto.CreateTraderRequest) (*model.Trader, *middleware.APIError) {
	if !validExchanges[in.Exchange] {
		return nil, middleware.BadRequest("unsupported exchange", map[string]string{"exchange": "must be one of binance,hyperliquid,aster,bybit,okx,bitget,gate,kucoin,indodax,lighter"})
	}
	if _, ok := svc.store.GetModel(in.ModelConfig.ModelID); !ok {
		return nil, middleware.BadRequest("referenced model not found", map[string]string{"model_config.model_id": "model id does not exist"})
	}
	if in.Schedule != nil && in.Schedule.Interval < 0 {
		return nil, middleware.BadRequest("schedule.interval must be >= 0", nil)
	}
	if in.Schedule != nil && len(in.Schedule.ActiveHours) > dto.MaxActiveHours {
		return nil, middleware.BadRequest("schedule.active_hours too many (max 24)", nil)
	}
	t := &model.Trader{
		UserID: userID,
		Name:        in.Name,
		Exchange:    in.Exchange,
		ModelConfig: model.ModelConfig{Provider: in.ModelConfig.Provider, ModelID: in.ModelConfig.ModelID, Parameters: in.ModelConfig.Parameters},
		StrategyID:  in.StrategyID,
		RiskConfig:  model.RiskConfig(in.RiskConfig),
		Status:      model.StatusIdle,
		Metrics:     model.Metrics{},
	}
	if in.Schedule != nil {
		t.Schedule = &model.Schedule{Interval: in.Schedule.Interval}
		for _, h := range in.Schedule.ActiveHours {
			t.Schedule.ActiveHours = append(t.Schedule.ActiveHours, model.ActiveHours{Start: h.Start, End: h.End})
		}
	}
	svc.store.CreateTrader(t)
	// L1：不能返回存活指针（handler 锁外序列化 vs WithTrader 锁内改写 = 数据竞争）。
	// 重新取深拷贝返回。
	created, ok := svc.store.GetTrader(t.ID)
	if !ok {
		return nil, middleware.Internal("trader created but not found")
	}
	return created, nil
}

// Update PATCH 部分更新（指针字段区分未传/置空）。
// L6：running 状态禁止修改配置（改后与引擎行为脱节）。
func (svc *TraderService) Update(id, userID string, in *dto.UpdateTraderRequest) (*model.Trader, *middleware.APIError) {
	// 锁外前置校验：WithTrader 回调持写锁，回调内调 GetModel 会写锁内取读锁死锁
	if in.Exchange != nil && !validExchanges[*in.Exchange] {
		return nil, middleware.BadRequest("unsupported exchange", nil)
	}
	if in.ModelConfig != nil {
		if _, ok := svc.store.GetModel(in.ModelConfig.ModelID); !ok {
			return nil, middleware.BadRequest("referenced model not found", nil)
		}
	}
	if in.Schedule != nil && in.Schedule.Interval < 0 {
		return nil, middleware.BadRequest("schedule.interval must be >= 0", nil)
	}
	if in.Schedule != nil && len(in.Schedule.ActiveHours) > dto.MaxActiveHours {
		return nil, middleware.BadRequest("schedule.active_hours too many (max 24)", nil)
	}

	updated, err := svc.store.WithTrader(id, func(t *model.Trader) error {
		if userID != "" && t.UserID != "" && t.UserID != userID {
			return store.ErrNotFound // 归属校验：他人交易员视同不存在
		}
		if t.Status == model.StatusRunning {
			return errRunningCannotModify
		}
		if in.Name != nil {
			t.Name = *in.Name
		}
		if in.Exchange != nil {
			t.Exchange = *in.Exchange
		}
		if in.ModelConfig != nil {
			t.ModelConfig = model.ModelConfig{Provider: in.ModelConfig.Provider, ModelID: in.ModelConfig.ModelID, Parameters: in.ModelConfig.Parameters}
		}
		if in.StrategyID != nil {
			t.StrategyID = *in.StrategyID
		}
		if in.RiskConfig != nil {
			t.RiskConfig = model.RiskConfig(*in.RiskConfig)
		}
		if in.Schedule != nil {
			sch := &model.Schedule{Interval: in.Schedule.Interval}
			for _, h := range in.Schedule.ActiveHours {
				sch.ActiveHours = append(sch.ActiveHours, model.ActiveHours{Start: h.Start, End: h.End})
			}
			t.Schedule = sch
		}
		return nil
	})
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, middleware.NotFound("trader not found")
	case errors.Is(err, errRunningCannotModify):
		return nil, middleware.BadRequest("cannot modify trader while running, stop it first", nil)
	case err != nil:
		return nil, middleware.Internal("update trader failed: " + err.Error())
	}
	return updated, nil
}

// Get 单个交易员。
func (svc *TraderService) Get(id, userID string) (*model.Trader, *middleware.APIError) {
	t, ok := svc.store.GetTrader(id)
	if !ok {
		return nil, middleware.NotFound("trader not found")
	}
	if userID != "" && t.UserID != "" && t.UserID != userID {
		return nil, middleware.NotFound("trader not found")
	}
	return t, nil
}

// List 全部交易员。
func (svc *TraderService) List(userID string) []*model.Trader {
	return svc.store.ListTraders(userID)
}

// Start 启动（幂等：running 时 1204；非法迁移 1004）。
// 状态迁移成功后挂载交易引擎；引擎启动失败回滚状态到 idle。
func (svc *TraderService) Start(id, userID string) *middleware.APIError {
	if err := svc.transition(id, model.StatusRunning, userID); err != nil {
		return err
	}
	if svc.engine != nil {
		if engErr := svc.engine.Start(id); engErr != nil {
			svc.store.WithTrader(id, func(t *model.Trader) error {
				t.Status = model.StatusIdle
				return nil
			})
			return middleware.Internal("engine start failed: " + engErr.Error())
		}
	}
	return nil
}

// Pause 暂停（仅 running→paused；引擎循环读 store 状态自动等待）。
func (svc *TraderService) Pause(id, userID string) *middleware.APIError {
	return svc.transition(id, model.StatusPaused, userID)
}

// Resume 恢复（仅 paused→running）。
func (svc *TraderService) Resume(id, userID string) *middleware.APIError {
	return svc.transition(id, model.StatusRunning, userID)
}

// Stop 停止（running|paused→stopped）。先停引擎（幂等）再迁移状态。
func (svc *TraderService) Stop(id, userID string) *middleware.APIError {
	if err := svc.transition(id, model.StatusStopped, userID); err != nil {
		return err
	}
	if svc.engine != nil {
		svc.engine.Stop(id)
	}
	return nil
}

func (svc *TraderService) transition(id, to, userID string) *middleware.APIError {
	// WithTrader：锁内原子读-改-写，并发 start/pause 同一交易员不会互相踩踏
	_, err := svc.store.WithTrader(id, func(t *model.Trader) error {
		if userID != "" && t.UserID != "" && t.UserID != userID {
			return store.ErrNotFound // 归属校验：他人交易员视同不存在
		}
		if t.Status == to {
			if to == model.StatusRunning {
				return errTraderRunning // 幂等：运行中重复 start -> 1204
			}
			return errAlreadyStopped
		}
		if !transitions[t.Status][to] {
			return errIllegalTransition // 状态机非法跳转 -> 1004（文档 §8）
		}
		t.Status = to
		return nil
	})
	switch {
	case errors.Is(err, store.ErrNotFound):
		return middleware.NotFound("trader not found")
	case errors.Is(err, errTraderRunning):
		return middleware.TraderRunning("trader is already running")
	case errors.Is(err, errIllegalTransition):
		return middleware.NotFound("illegal state transition: -> " + to)
	case errors.Is(err, errAlreadyStopped):
		return middleware.BadRequest("trader is already "+to, nil)
	case err != nil:
		// L8：内部错误必须用 1500（此前 1001 配 HTTP 500，前端误走字段高亮分支）
		return middleware.Internal("state transition failed: " + err.Error())
	}
	return nil
}

// 状态迁移哨兵错误（WithTrader 回调内返回）。
var (
	errTraderRunning     = errors.New("trader already running")
	errIllegalTransition  = errors.New("illegal state transition")
	errAlreadyStopped    = errors.New("trader already in target state")
	errRunningCannotModify = errors.New("cannot modify trader while running")
)

// ListPositions 持仓列表（支持 symbol 过滤）。
func (svc *TraderService) ListPositions(userID, symbol string) []*model.Position {
	all := svc.store.ListPositions()
	owner := map[string]string{} // traderID -> userID（缓存）
	out := make([]*model.Position, 0, len(all))
	for _, p := range all {
		if userID != "" {
			uid, ok := owner[p.TraderID]
			if !ok {
				if t, found := svc.store.GetTrader(p.TraderID); found {
					uid = t.UserID
				}
				owner[p.TraderID] = uid
			}
			// 非本用户持仓跳过（uid 空 = 历史数据放行）
			if uid != "" && uid != userID {
				continue
			}
		}
		if symbol == "" || p.Symbol == symbol {
			out = append(out, p)
		}
	}
	return out
}

// ClosePosition 平仓并回写交易员指标。
// L2：拒绝 NaN/Inf（非有限值会永久污染聚合指标）。
// L1：胜率重算在写锁内完成（WithTraderAndPositions 提供锁内持仓快照），
// 并发平仓不会基于过期快照互相覆盖（锁外算胜率已实测复现 WinRate 失真）。
func (svc *TraderService) ClosePosition(userID, id string, pnl float64) (*model.Position, *middleware.APIError) {
	if math.IsNaN(pnl) || math.IsInf(pnl, 0) {
		return nil, middleware.BadRequest("pnl must be a finite number", map[string]string{"pnl": "must be finite"})
	}
	// 归属校验：持仓所属 trader 必须属于当前用户（越权平仓拒绝）
	if existing, found := svc.store.GetPosition(id); found {
		if t, ok := svc.store.GetTrader(existing.TraderID); ok && userID != "" && t.UserID != "" && t.UserID != userID {
			return nil, middleware.NotFound("position not found")
		}
	}
	p, closed := svc.store.ClosePosition(id, pnl)
	if !closed {
		if existing, found := svc.store.GetPosition(id); found && existing.ClosedAt != nil {
			return nil, middleware.BadRequest("position already closed", nil)
		}
		return nil, middleware.NotFound("position not found")
	}
	_, err := svc.store.WithTraderAndPositions(p.TraderID, func(t *model.Trader, positions []*model.Position) error {
		m := t.Metrics
		m.TradeCount++
		m.TotalPnL += pnl
		m.DailyPnL += pnl
		m.WinRate = calcWinRate(positions, p.TraderID)
		t.Metrics = m
		return nil
	})
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, middleware.Internal("update trader metrics failed: " + err.Error())
	}
	return p, nil
}

// calcWinRate 按持仓快照重算胜率（PnL>0 已平仓占比）。
// 必须在写锁内调用（由 WithTraderAndPositions 提供锁内快照，L1）。
func calcWinRate(positions []*model.Position, traderID string) float64 {
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

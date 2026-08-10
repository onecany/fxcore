package service

import (
	"math"
	"sync"
	"testing"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "x", AdminSignSecret: "s"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func seedTrader(t *testing.T, s *store.Store, id string) {
	t.Helper()
	m := &model.AIModel{ID: "m1", Provider: "deepseek", ModelName: "deepseek-chat", APIKeyEnc: "enc", Status: "active"}
	s.CreateModel(m)
	trader := &model.Trader{
		ID:          id,
		Name:        "t",
		Exchange:    "binance",
		ModelConfig: model.ModelConfig{ModelID: "m1", Provider: "deepseek"},
		StrategyID:  "grid-1",
		RiskConfig:  model.RiskConfig{MaxPositionSize: 100},
		Status:      model.StatusIdle,
	}
	s.CreateTrader(trader)
}

// 并发 Start 同一交易员：恰好 1 次成功，其余全部 1204（原子迁移，无踩踏）。
func TestConcurrentStartOnlyOneSucceeds(t *testing.T) {
	s := newTestStore(t)
	seedTrader(t, s, "t1")
	svc := NewTraderService(s, nil)

	const n = 24
	errs := make([]*middleware.APIError, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.Start("t1", "u1")
		}(i)
	}
	wg.Wait()

	success, running := 0, 0
	for _, e := range errs {
		switch {
		case e == nil:
			success++
		case e.Code == middleware.CodeTraderRunning:
			running++
		default:
			t.Fatalf("unexpected error code %d: %s", e.Code, e.Message)
		}
	}
	if success != 1 {
		t.Fatalf("expected exactly 1 success, got %d", success)
	}
	if running != n-1 {
		t.Fatalf("expected %d x 1204, got %d", n-1, running)
	}

	trader, _ := s.GetTrader("t1")
	if trader.Status != model.StatusRunning {
		t.Fatalf("final status = %s, want running", trader.Status)
	}
}

// 并发 停→启 混跑：不 panic、状态始终合法（running|stopped），race detector 覆盖。
func TestConcurrentStopStartNoRace(t *testing.T) {
	s := newTestStore(t)
	seedTrader(t, s, "t2")
	svc := NewTraderService(s, nil)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_ = svc.Start("t2", "u1")
			} else {
				_ = svc.Stop("t2", "u1")
			}
		}(i)
	}
	wg.Wait()

	trader, _ := s.GetTrader("t2")
	if trader.Status != model.StatusRunning && trader.Status != model.StatusStopped {
		t.Fatalf("final status = %s, want running or stopped", trader.Status)
	}
}

// 非法迁移：idle 直接 pause -> 1004；paused 时 resume 合法。
func TestTransitionRules(t *testing.T) {
	s := newTestStore(t)
	seedTrader(t, s, "t3")
	svc := NewTraderService(s, nil)

	if e := svc.Pause("t3", "u1"); e == nil || e.Code != middleware.CodeNotFound {
		t.Fatalf("idle->paused should be 1004, got %+v", e)
	}
	if e := svc.Start("t3", "u1"); e != nil {
		t.Fatalf("idle->running should succeed, got %+v", e)
	}
	if e := svc.Start("t3", "u1"); e == nil || e.Code != middleware.CodeTraderRunning {
		t.Fatalf("running->running start should be 1204, got %+v", e)
	}
	if e := svc.Pause("t3", "u1"); e != nil {
		t.Fatalf("running->paused should succeed, got %+v", e)
	}
	if e := svc.Start("t3", "u1"); e != nil {
		t.Fatalf("paused start should succeed (restart semantics), got %+v", e)
	}
	if e := svc.Stop("t3", "u1"); e != nil {
		t.Fatalf("running->stopped should succeed, got %+v", e)
	}
	if e := svc.Start("t3", "u1"); e != nil {
		t.Fatalf("stopped->running restart should succeed, got %+v", e)
	}
}

// store 读返回副本：外部修改不影响内部状态。
func TestStoreReturnsCopies(t *testing.T) {
	s := newTestStore(t)
	seedTrader(t, s, "t4")

	cp, _ := s.GetTrader("t4")
	cp.Status = model.StatusStopped // 外部改副本

	again, _ := s.GetTrader("t4")
	if again.Status != model.StatusIdle {
		t.Fatalf("internal state mutated via returned copy: %s", again.Status)
	}
}

// L1：并发平仓两条（-1 / +1），胜率必须在写锁内重算 —— 最终 WinRate=0.5。
func TestConcurrentClosePositionsWinRate(t *testing.T) {
	s := newTestStore(t)
	seedTrader(t, s, "t5")
	svc := NewTraderService(s, nil)
	s.AddPosition(&model.Position{Symbol: "BTC-USDT", Side: "long", Size: 1, EntryPrice: 100, TraderID: "t5"})
	s.AddPosition(&model.Position{Symbol: "BTC-USDT", Side: "short", Size: 1, EntryPrice: 100, TraderID: "t5"})
	ids := s.ListPositions()
	if len(ids) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(ids))
	}

	errs := make([]*middleware.APIError, 2)
	var wg sync.WaitGroup
	for i, p := range ids {
		wg.Add(1)
		go func(i int, id string, pnl float64) {
			defer wg.Done()
			_, errs[i] = svc.ClosePosition("u1", id, pnl)
		}(i, p.ID, float64(i)*2-1) // p0: -1, p1: +1
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("close #%d failed: %+v", i, e)
		}
	}

	trader, _ := s.GetTrader("t5")
	if trader.Metrics.TradeCount != 2 {
		t.Fatalf("TradeCount=%d, want 2", trader.Metrics.TradeCount)
	}
	if trader.Metrics.TotalPnL != 0 {
		t.Fatalf("TotalPnL=%v, want 0", trader.Metrics.TotalPnL)
	}
	if trader.Metrics.WinRate != 0.5 {
		t.Fatalf("WinRate=%v, want 0.5 (lock-race regression)", trader.Metrics.WinRate)
	}
}

// L2：NaN/Inf pnl 必须拒绝（非有限值会永久污染聚合指标）。
func TestClosePositionRejectsNaN(t *testing.T) {
	s := newTestStore(t)
	seedTrader(t, s, "t6")
	svc := NewTraderService(s, nil)
	s.AddPosition(&model.Position{Symbol: "BTC-USDT", Side: "long", Size: 1, EntryPrice: 100, TraderID: "t6"})
	p := s.ListPositions()[0]

	if _, e := svc.ClosePosition("u1", p.ID, math.NaN()); e == nil || e.Code != middleware.CodeBadRequest {
		t.Fatalf("NaN pnl should be 1001, got %+v", e)
	}
	if _, e := svc.ClosePosition("u1", p.ID, math.Inf(1)); e == nil || e.Code != middleware.CodeBadRequest {
		t.Fatalf("+Inf pnl should be 1001, got %+v", e)
	}
	if _, e := svc.ClosePosition("u1", p.ID, math.Inf(-1)); e == nil || e.Code != middleware.CodeBadRequest {
		t.Fatalf("-Inf pnl should be 1001, got %+v", e)
	}
}

// L6：running 状态禁止 PATCH 修改配置。
func TestUpdateRejectedWhileRunning(t *testing.T) {
	s := newTestStore(t)
	seedTrader(t, s, "t7")
	svc := NewTraderService(s, nil)
	if e := svc.Start("t7", "u1"); e != nil {
		t.Fatalf("start failed: %+v", e)
	}
	newName := "renamed"
	if _, e := svc.Update("t7", "u1", &dto.UpdateTraderRequest{Name: &newName}); e == nil || e.Code != middleware.CodeBadRequest {
		t.Fatalf("update while running should be 1001, got %+v", e)
	}
	if e := svc.Stop("t7", "u1"); e != nil {
		t.Fatalf("stop failed: %+v", e)
	}
	if _, e := svc.Update("t7", "u1", &dto.UpdateTraderRequest{Name: &newName}); e != nil {
		t.Fatalf("update after stop should succeed, got %+v", e)
	}
}

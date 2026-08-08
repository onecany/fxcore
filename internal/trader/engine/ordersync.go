package engine

import (
	"context"
	"time"

	"fxcore/internal/exchange"
	"fxcore/internal/model"
)

// OrderSync + PositionBuilder（API设计.md §14.1）：
// 每交易员 30s goroutine 拉 24h 成交 → 按 exchange_trade_id 去重 →
// 写 order + fill；PositionBuilder 是 position 表唯一写者（引擎直接下单
// 的即时落库除外——OrderSync 负责交易所侧成交的校正/补录）。
// 落库幂等：AddOrder/AddFill 内部按唯一键去重。

// orderSyncLoop 每 30s 同步一次成交。
func (e *Engine) orderSyncLoop(ctx context.Context, traderID string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		t, ok := e.store.GetTrader(traderID)
		if !ok || t.Status == model.StatusStopped || t.Status == model.StatusError {
			return
		}
		if err := e.orderSync(ctx, t); err != nil {
			e.noteTraderError(traderID, "ordersync: "+err.Error())
		}
	}
}

// orderSync 拉取 24h 成交并幂等落库。
func (e *Engine) orderSync(ctx context.Context, t *model.Trader) error {
	adapter, err := e.adapterFor(t)
	if err != nil {
		return err
	}
	since := time.Now().Add(-24 * time.Hour).UnixMilli()
	fills, err := adapter.GetFills(ctx, since)
	if err != nil {
		return err
	}
	for _, f := range fills {
		// 幂等落 fill（exchange_trade_id 唯一，重复跳过）
		fill, added := e.store.AddFill(&model.Fill{
			TraderID:        t.ID,
			ExchangeOrderID: f.ExchangeOrderID,
			ExchangeTradeID: f.ExchangeTradeID,
			Symbol:          f.Symbol,
			Side:            f.Side,
			Price:           f.Price,
			Quantity:        f.Quantity,
			Commission:      f.Commission,
			CommissionAsset: f.CommissionAsset,
			RealizedPnL:     f.RealizedPnL,
			IsMaker:         f.IsMaker,
		})
		if !added {
			continue // 已存在
		}
		// 幂等落 order（exchange_order_id 唯一）
		order, orderAdded := e.store.AddOrder(&model.Order{
			TraderID:        t.ID,
			ExchangeOrderID: f.ExchangeOrderID,
			Symbol:          f.Symbol,
			Side:            f.Side,
			Type:            "market",
			Quantity:        f.Quantity,
			Status:          "FILLED",
			FilledQuantity:  f.Quantity,
			AvgFillPrice:    f.Price,
			Commission:      f.Commission,
			CommissionAsset: f.CommissionAsset,
		})
		if !orderAdded {
			continue
		}
		_ = order
		_ = fill
		// PositionBuilder：成交校正本地持仓（开仓补录 / 平仓结算）
		e.positionBuilder(t.ID, f)
	}
	return nil
}

// positionBuilder 以成交为事实源校正持仓（§14.1：position 表唯一写者语义）。
// 简化实现：开仓成交补录 OPEN 持仓（本地已存在则跳过）；平仓成交结算本地持仓。
func (e *Engine) positionBuilder(traderID string, f exchange.Fill) {
	open := e.store.ListPositionsByTrader(traderID)
	side := "long"
	if f.Side == "sell" {
		side = "short"
	}
	// 平仓侧：find 同 symbol 的 OPEN 持仓（方向匹配）→ 结算
	var matched *model.Position
	for _, p := range open {
		if p.Symbol == f.Symbol && p.Side == side && p.ClosedAt == nil {
			matched = p
			break
		}
	}
	if matched != nil {
		// 成交数量 ≥ 持仓数量 → 全平；否则减仓（简化：全平）
		if f.Quantity >= matched.Quantity-1e-9 {
			e.store.ClosePosition(matched.ID, f.RealizedPnL)
			// 指标回写（同 closePosition 语义）
			e.store.WithTraderAndPositions(traderID, func(t *model.Trader, all []*model.Position) error {
				m := t.Metrics
				m.TradeCount++
				m.TotalPnL += f.RealizedPnL
				m.DailyPnL += f.RealizedPnL
				m.WinRate = calcWinRateLocal(all, traderID)
				t.Metrics = m
				return nil
			})
		}
		return
	}
	// 开仓侧：本地无持仓则补录（引擎直接下单已落库的会跳过）
	for _, p := range open {
		if p.Symbol == f.Symbol && p.Status == model.PositionOpen {
			return // 已有本地持仓，跳过补录
		}
	}
	e.store.AddPosition(&model.Position{
		TraderID:   traderID,
		Symbol:     f.Symbol,
		Side:       side,
		Size:       f.Quantity,
		Quantity:   f.Quantity,
		EntryPrice: f.Price,
		MarkPrice:  f.Price,
		Status:     model.PositionOpen,
		Source:     "ordersync",
	})
}

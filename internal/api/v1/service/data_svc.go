package service

import (
	"context"
	"math"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/provider"
	"fxcore/internal/store"
)

// DataService 数据/市场只读端点（API设计.md §11 data + market 路由）。
// 无引擎依赖的部分从 store 聚合真实数据；open-orders/competition/top-traders
// 依赖交易引擎（阶段 3d）返回空骨架，klines 已接数据源链。
type DataService struct {
	store  *store.Store
	klines provider.KlineProvider
}

// NewDataService 构造数据服务。
func NewDataService(s *store.Store, klines provider.KlineProvider) *DataService {
	return &DataService{store: s, klines: klines}
}

// Status 运行状态。
func (svc *DataService) Status(traderID, userID string) dto.StatusDTO {
	out := dto.StatusDTO{TraderID: traderID}
	if t, ok := svc.store.GetTrader(traderID); ok {
		out.IsRunning = t.Status == model.StatusRunning
	}
	return out
}

// Account 账户信息：最新权益快照 + 当前持仓数聚合。
func (svc *DataService) Account(traderID, userID string) dto.AccountInfoDTO {
	out := dto.AccountInfoDTO{Currency: "USDT"}
	equities := svc.store.ListEquityByTrader(traderID, userID)
	if n := len(equities); n > 0 {
		last := equities[n-1]
		out.TotalEquity = last.TotalEquity
		out.AvailableBalance = last.Balance
		out.UnrealizedPnL = last.UnrealizedPnL
		out.MarginUsedPct = last.MarginUsedPct
	}
	out.PositionCount = len(svc.store.ListPositionsByTrader(traderID))
	return out
}

// Decisions 决策记录分页（最新在前）。
func (svc *DataService) Decisions(traderID, userID string, page, size int) ([]*model.DecisionRecord, int) {
	total := svc.store.CountDecisions(traderID, userID)
	offset := (page - 1) * size
	if offset > total {
		offset = total
	}
	items := svc.store.ListDecisionsByTrader(traderID, userID, offset, size)
	return items, total
}

// LatestDecision 最近一轮决策。
func (svc *DataService) LatestDecision(traderID, userID string) (*model.DecisionRecord, bool) {
	return svc.store.LatestDecisionByTrader(traderID)
}

// Statistics 交易统计：从平仓历史计算胜率/盈亏因子（store 平仓记录为准）。
func (svc *DataService) Statistics(traderID, userID string) dto.StatisticsDTO {
	closed := svc.store.ListPositionHistory(traderID, "", "", 0, 100000)
	out := dto.StatisticsDTO{TotalTrades: len(closed)}
	if len(closed) == 0 {
		return out
	}
	var wins, losses int
	var grossWin, grossLoss, totalPnL float64
	for _, p := range closed {
		totalPnL += p.PnL
		if p.PnL > 0 {
			wins++
			grossWin += p.PnL
		} else if p.PnL < 0 {
			losses++
			grossLoss += p.PnL
		}
	}
	if len(closed) > 0 {
		out.WinRate = float64(wins) / float64(len(closed))
	}
	out.TotalPnL = totalPnL
	if losses > 0 && grossLoss != 0 {
		out.ProfitFactor = grossWin / math.Abs(grossLoss)
	}
	if wins > 0 {
		out.AvgWin = grossWin / float64(wins)
	}
	if losses > 0 {
		out.AvgLoss = grossLoss / float64(losses)
	}
	return out
}

// Trades 成交事件分页（fills 最新在前）。
func (svc *DataService) Trades(traderID, userID string, page, size int) []*model.Fill {
	all := svc.store.ListFillsByTrader(traderID, userID)
	start := (page - 1) * size
	if start > len(all) {
		start = len(all)
	}
	end := start + size
	if end > len(all) {
		end = len(all)
	}
	return all[start:end]
}

// CountTrades 成交总数。
func (svc *DataService) CountTrades(traderID, userID string) int {
	return len(svc.store.ListFillsByTrader(traderID, userID))
}

// Orders 订单分页。
func (svc *DataService) Orders(traderID, userID string, page, size int) []*model.Order {
	all := svc.store.ListOrders(traderID, userID)
	start := (page - 1) * size
	if start > len(all) {
		start = len(all)
	}
	end := start + size
	if end > len(all) {
		end = len(all)
	}
	return all[start:end]
}

// CountOrders 订单总数。
func (svc *DataService) CountOrders(traderID, userID string) int {
	return len(svc.store.ListOrders(traderID, userID))
}

// OrderFills 单笔订单的成交明细。
func (svc *DataService) OrderFills(orderID string) []*model.Fill {
	return svc.store.ListFillsByOrder(orderID)
}

// OpenOrders 交易所实时挂单。骨架：引擎层（阶段 3）接入交易所适配器后填充。
func (svc *DataService) OpenOrders(traderID string) []*model.Order {
	_ = traderID
	return []*model.Order{}
}

// Klines K 线行情（数据源链：hyperliquid→okx→coinank）。
func (svc *DataService) Klines(symbol, interval string, limit int) ([]dto.KlineDTO, error) {
	if svc.klines == nil {
		return []dto.KlineDTO{}, nil
	}
	klines, err := svc.klines.Klines(context.Background(), symbol, interval, limit)
	if err != nil {
		return nil, err
	}
	return klines, nil
}

// Symbols 交易对列表。骨架：provider 就位后填充。
func (svc *DataService) Symbols() []string {
	return []string{}
}

// EquityHistory 权益曲线（时间升序）。
func (svc *DataService) EquityHistory(traderID, userID string) []*model.EquitySnapshot {
	return svc.store.ListEquityByTrader(traderID, userID)
}

// TraderPublicConfig 脱敏公开配置。
func (svc *DataService) TraderPublicConfig(id string) (*model.Trader, *middleware.APIError) {
	t, ok := svc.store.GetTrader(id)
	if !ok {
		return nil, middleware.NotFound("trader not found")
	}
	return t, nil
}

// Store 暴露底层 store（handler 直接读平仓历史等聚合查询）。
func (svc *DataService) Store() *store.Store {
	return svc.store
}

// Competition 公开市场数据。骨架：引擎层就位后填充。
func (svc *DataService) Competition() any {
	return map[string]any{
		"total_traders": 0,
		"total_pnl":     0,
		"rankings":      []any{},
	}
}

// TopTraders 排行榜。骨架：引擎层就位后填充。
func (svc *DataService) TopTraders() []*model.Trader {
	return []*model.Trader{}
}

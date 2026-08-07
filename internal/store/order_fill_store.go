package store

import (
	"time"

	"fxcore/internal/model"
)

// ========== 订单（幂等追加，ExchangeOrderID 去重） ==========

// AddOrder 落订单。幂等契约（API设计.md §14.1）：同 exchange_order_id 重复写入
// 返回已有记录（交易所侧重放/OrderSync 拉取去重均依赖此语义），不产生重复行。
func (s *Store) AddOrder(o *model.Order) (*model.Order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o.ExchangeOrderID != "" {
		for _, existing := range s.orders {
			if existing.ExchangeOrderID == o.ExchangeOrderID {
				cp := *existing
				return &cp, false // 已存在，幂等跳过
			}
		}
	}
	if o.ID == "" {
		o.ID = newID()
	}
	o.CreatedAt = time.Now().UTC()
	o.UpdatedAt = o.CreatedAt
	s.orders = append(s.orders, o)
	return o, true
}

// GetOrder 按 ID 查订单（返回副本）。
func (s *Store) GetOrder(id string) (*model.Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, o := range s.orders {
		if o.ID == id {
			cp := *o
			return &cp, true
		}
	}
	return nil, false
}

// ListOrders 按 trader 列出订单（返回副本切片，最新在前）。
func (s *Store) ListOrders(traderID string) []*model.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Order, 0, 16)
	for i := len(s.orders) - 1; i >= 0; i-- {
		o := s.orders[i]
		if traderID != "" && o.TraderID != traderID {
			continue
		}
		cp := *o
		out = append(out, &cp)
	}
	return out
}

// ========== 成交（幂等追加，ExchangeTradeID 去重） ==========

// AddFill 落成交。幂等契约同 AddOrder：exchange_trade_id 重复写入返回已有记录。
func (s *Store) AddFill(f *model.Fill) (*model.Fill, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.ExchangeTradeID != "" {
		for _, existing := range s.fills {
			if existing.ExchangeTradeID == f.ExchangeTradeID {
				cp := *existing
				return &cp, false
			}
		}
	}
	if f.ID == "" {
		f.ID = newID()
	}
	f.CreatedAt = time.Now().UTC()
	s.fills = append(s.fills, f)
	return f, true
}

// ListFillsByOrder 按订单列成交（返回副本切片）。
func (s *Store) ListFillsByOrder(orderID string) []*model.Fill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Fill, 0, 8)
	for _, f := range s.fills {
		if f.OrderID != orderID {
			continue
		}
		cp := *f
		out = append(out, &cp)
	}
	return out
}

// ListFillsByTrader 按 trader 列成交（返回副本切片，最新在前）。
func (s *Store) ListFillsByTrader(traderID string) []*model.Fill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Fill, 0, 16)
	for i := len(s.fills) - 1; i >= 0; i-- {
		f := s.fills[i]
		if traderID != "" && f.TraderID != traderID {
			continue
		}
		cp := *f
		out = append(out, &cp)
	}
	return out
}

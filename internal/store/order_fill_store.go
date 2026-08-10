package store

import (
	"time"

	"fxcore/internal/model"
)

// ========== 订单（幂等追加，ExchangeOrderID 去重） ==========

// AddOrder 落订单。幂等契约（API设计.md §14.1）：同 exchange_order_id 重复写入
// 返回已有记录（交易所侧重放/OrderSync 拉取去重均依赖此语义），不产生重复行。
// DB 路径：uniqueIndex 冲突 → 返回既有行（不报错、不重复插入）。
func (s *Store) AddOrder(o *model.Order) (*model.Order, bool) {
	if s.db != nil {
		if o.ExchangeOrderID != "" {
			var existing model.Order
			if err := s.db.Where("exchange_order_id = ?", o.ExchangeOrderID).First(&existing).Error; err == nil {
				return &existing, false // 幂等命中
			}
		}
		if o.ID == "" {
			o.ID = newID()
		}
		o.CreatedAt = time.Now().UTC()
		o.UpdatedAt = o.CreatedAt
		if err := s.db.Create(o).Error; err != nil {
			// 冲突/忙等失败统一兜底：重查既有行（幂等语义优先），查不到才报失败
			if o.ExchangeOrderID != "" {
				var existing model.Order
				if e2 := s.db.Where("exchange_order_id = ?", o.ExchangeOrderID).First(&existing).Error; e2 == nil {
					return &existing, false
				}
			}
			return nil, false
		}
		s.mu.Lock()
		s.orders = append(s.orders, o)
		s.mu.Unlock()
		return o, true
	}
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
	if s.db != nil {
		var o model.Order
		if err := s.db.First(&o, "id = ?", id).Error; err != nil {
			return nil, false
		}
		return &o, true
	}
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
func (s *Store) ListOrders(traderID, userID string) []*model.Order {
	if s.db != nil {
		q := s.db.Model(&model.Order{})
		if traderID != "" {
			q = q.Where("trader_id = ?", traderID)
		} else if userID != "" {
			q = q.Where("trader_id IN (SELECT id FROM traders WHERE user_id = ?)", userID)
		}
		var rows []model.Order
		q.Order("created_at DESC").Find(&rows)
		out := make([]*model.Order, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	owner := map[string]string{}
	out := make([]*model.Order, 0, 16)
	for i := len(s.orders) - 1; i >= 0; i-- {
		o := s.orders[i]
		if traderID != "" && o.TraderID != traderID {
			continue
		}
		if traderID == "" && userID != "" {
			uid, ok := owner[o.TraderID]
			if !ok {
				if t, found := s.traders[o.TraderID]; found {
					uid = t.UserID
				}
				owner[o.TraderID] = uid
			}
			if uid != "" && uid != userID {
				continue
			}
		}
		cp := *o
		out = append(out, &cp)
	}
	return out
}

// ========== 成交（幂等追加，ExchangeTradeID 去重） ==========

// AddFill 落成交。幂等契约同 AddOrder：exchange_trade_id 重复写入返回已有记录。
func (s *Store) AddFill(f *model.Fill) (*model.Fill, bool) {
	if s.db != nil {
		if f.ExchangeTradeID != "" {
			var existing model.Fill
			if err := s.db.Where("exchange_trade_id = ?", f.ExchangeTradeID).First(&existing).Error; err == nil {
				return &existing, false // 幂等命中
			}
		}
		if f.ID == "" {
			f.ID = newID()
		}
		f.CreatedAt = time.Now().UTC()
		if err := s.db.Create(f).Error; err != nil {
			// 冲突/忙等失败统一兜底：重查既有行（幂等语义优先），查不到才报失败
			if f.ExchangeTradeID != "" {
				var existing model.Fill
				if e2 := s.db.Where("exchange_trade_id = ?", f.ExchangeTradeID).First(&existing).Error; e2 == nil {
					return &existing, false
				}
			}
			return nil, false
		}
		s.mu.Lock()
		s.fills = append(s.fills, f)
		s.mu.Unlock()
		return f, true
	}
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
	if s.db != nil {
		var rows []model.Fill
		s.db.Where("order_id = ?", orderID).Order("created_at ASC").Find(&rows)
		out := make([]*model.Fill, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
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
func (s *Store) ListFillsByTrader(traderID, userID string) []*model.Fill {
	if s.db != nil {
		q := s.db.Model(&model.Fill{})
		if traderID != "" {
			q = q.Where("trader_id = ?", traderID)
		} else if userID != "" {
			q = q.Where("trader_id IN (SELECT id FROM traders WHERE user_id = ?)", userID)
		}
		var rows []model.Fill
		q.Order("created_at DESC").Find(&rows)
		out := make([]*model.Fill, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	owner := map[string]string{}
	out := make([]*model.Fill, 0, 16)
	for i := len(s.fills) - 1; i >= 0; i-- {
		f := s.fills[i]
		if traderID != "" && f.TraderID != traderID {
			continue
		}
		if traderID == "" && userID != "" {
			uid, ok := owner[f.TraderID]
			if !ok {
				if t, found := s.traders[f.TraderID]; found {
					uid = t.UserID
				}
				owner[f.TraderID] = uid
			}
			if uid != "" && uid != userID {
				continue
			}
		}
		cp := *f
		out = append(out, &cp)
	}
	return out
}

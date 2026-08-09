// Package store 提供进程内内存存储（单机、mutex 保护）。
// 设计说明：API设计.md 7.1 蓝图要求 internal/model/ 为 GORM 实体、
// v1/v2 共享。当前交付以内存实现打通全链路，实体已 GORM-ready，
// 后续接 SQLite/PostgreSQL 时替换本包实现即可，接口不变。
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"fxcore/internal/model"
)

// ErrNotFound 存储未命中。
var ErrNotFound = errors.New("store: not found")

// Store 进程内存储。
// 并发模型：读方法返回结构体副本（调用方可安全持有），
// 写方法在写锁内完成；状态迁移类操作必须走 WithTrader（锁内原子读改写）。
// 持久化：db 非 nil 时管理实体（users/exchanges/strategies 等）走 GORM，
// 流式运行时数据（positions/orders/decisions/equities/backtest*/debate*）始终在内存。
type Store struct {
	db         *gorm.DB // nil = 纯内存模式（测试/无盘运行）
	mu         sync.RWMutex
	users      map[string]*model.User // by user id
	emailIndex map[string]string      // email -> user id
	models     map[string]*model.AIModel
	traders    map[string]*model.Trader
	positions  []*model.Position // 追加序，最新在末尾
	posSeq     int64
	// ---- §10-§15 扩展实体 ----
	exchanges  map[string]*model.Exchange // by id
	strategies map[string]*model.Strategy // by id
	telegram   *model.TelegramConfig      // 单用户单例
	orders     []*model.Order             // 追加序，最新在末尾
	fills      []*model.Fill              // 追加序，最新在末尾
	decisions  []*model.DecisionRecord    // 追加序，最新在末尾
	equities   []*model.EquitySnapshot    // 追加序，最新在末尾
	// ---- 回测 ----
	backtestRuns        map[string]*model.BacktestRun
	backtestEquities    []*model.BacktestEquity
	backtestTrades      []*model.BacktestTrade
	backtestDecisions   []*model.BacktestDecision
	backtestCheckpoints map[string]*model.BacktestCheckpoint
	// ---- 辩论 ----
	debateSessions     map[string]*model.DebateSession
	debateParticipants []*model.DebateParticipant
	debateMessages     []*model.DebateMessage
	debateVotes        []*model.DebateVote
}

// Config 初始化配置。
type Config struct {
	AdminEmail      string
	AdminPassword   string
	AdminSignSecret string // 为空则自动生成
}

// New 初始化存储并播种管理员账号。
// db 非 nil 时管理实体走 GORM（SQLite/MariaDB），nil 时纯内存（测试）。
func New(cfg Config, db *gorm.DB) (*Store, error) {
	s := &Store{
		db:           db,
		users:        make(map[string]*model.User),
		emailIndex:   make(map[string]string),
		models:       make(map[string]*model.AIModel),
		traders:      make(map[string]*model.Trader),
		positions:    make([]*model.Position, 0, 16),
		exchanges:    make(map[string]*model.Exchange),
		strategies:   make(map[string]*model.Strategy),
		orders:       make([]*model.Order, 0, 16),
		fills:        make([]*model.Fill, 0, 16),
		decisions:    make([]*model.DecisionRecord, 0, 16),
		equities:     make([]*model.EquitySnapshot, 0, 16),
		backtestRuns: make(map[string]*model.BacktestRun),
		backtestEquities:    make([]*model.BacktestEquity, 0, 16),
		backtestTrades:      make([]*model.BacktestTrade, 0, 16),
		backtestDecisions:   make([]*model.BacktestDecision, 0, 16),
		backtestCheckpoints: make(map[string]*model.BacktestCheckpoint),
		debateSessions:      make(map[string]*model.DebateSession),
		debateParticipants:  make([]*model.DebateParticipant, 0, 8),
		debateMessages:      make([]*model.DebateMessage, 0, 32),
		debateVotes:         make([]*model.DebateVote, 0, 8),
	}
	if err := s.seedAdmin(cfg); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) seedAdmin(cfg Config) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	secret := cfg.AdminSignSecret
	if secret == "" {
		secret = newID()
	}
	now := time.Now().UTC()
	u := &model.User{
		ID:           newID(),
		Email:        cfg.AdminEmail,
		PasswordHash: string(hash),
		Nickname:     "admin",
		SignSecret:   secret,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if s.db != nil {
		// 幂等：已存在（重启加载）则跳过，不重复播种
		var count int64
		s.db.Model(&model.User{}).Where("email = ?", cfg.AdminEmail).Count(&count)
		if count == 0 {
			if err := s.db.Create(u).Error; err != nil {
				return err
			}
		}
		// 同步内存索引（保证 GetUserByEmail 之外的直接索引路径一致）
		s.mu.Lock()
		s.users[u.ID] = u
		s.emailIndex[u.Email] = u.ID
		s.mu.Unlock()
		return nil
	}
	s.users[u.ID] = u
	s.emailIndex[u.Email] = u.ID
	return nil
}

// ========== 用户 ==========

// GetUserByEmail 按邮箱查用户（返回副本）。
func (s *Store) GetUserByEmail(email string) (*model.User, bool) {
	if s.db != nil {
		var u model.User
		if err := s.db.Where("email = ?", email).First(&u).Error; err != nil {
			return nil, false
		}
		return &u, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.emailIndex[email]
	if !ok {
		return nil, false
	}
	u, ok := s.users[id]
	if !ok {
		return nil, false
	}
	cp := *u
	return &cp, true
}

// GetUserByID 按 ID 查用户（返回副本）。
func (s *Store) GetUserByID(id string) (*model.User, bool) {
	if s.db != nil {
		var u model.User
		if err := s.db.First(&u, "id = ?", id).Error; err != nil {
			return nil, false
		}
		return &u, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, false
	}
	cp := *u
	return &cp, true
}

// ========== AI 模型 ==========

// CreateModel 创建模型（APIKeyEnc 已加密）。
func (s *Store) CreateModel(m *model.AIModel) {
	if m.ID == "" {
		m.ID = newID()
	}
	m.CreatedAt = time.Now().UTC()
	m.UpdatedAt = m.CreatedAt
	if s.db != nil {
		s.db.Create(m)
		s.mu.Lock()
		s.models[m.ID] = m
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.models[m.ID] = m
}

// GetModel 按 ID 查未删除模型（返回副本）。
func (s *Store) GetModel(id string) (*model.AIModel, bool) {
	if s.db != nil {
		var m model.AIModel
		if err := s.db.First(&m, "id = ?", id).Error; err != nil {
			return nil, false
		}
		return &m, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.models[id]
	if !ok || m.DeletedAt != nil {
		return nil, false
	}
	cp := *m
	return &cp, true
}

// ListModels 列出全部未删除模型（返回副本切片）。
func (s *Store) ListModels() []*model.AIModel {
	if s.db != nil {
		var rows []model.AIModel
		s.db.Order("created_at DESC").Find(&rows)
		out := make([]*model.AIModel, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.AIModel, 0, len(s.models))
	for _, m := range s.models {
		if m.DeletedAt == nil {
			cp := *m
			out = append(out, &cp)
		}
	}
	return out
}

// UpdateModel 更新模型。
func (s *Store) UpdateModel(m *model.AIModel) {
	m.UpdatedAt = time.Now().UTC()
	if s.db != nil {
		s.db.Save(m)
		s.mu.Lock()
		s.models[m.ID] = m
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.models[m.ID] = m
}

// DeleteModel 软删除，返回是否命中。
func (s *Store) DeleteModel(id string) bool {
	if s.db != nil {
		res := s.db.Delete(&model.AIModel{}, "id = ?", id)
		if res.RowsAffected > 0 {
			s.mu.Lock()
			if m, ok := s.models[id]; ok {
				now := time.Now().UTC()
				m.DeletedAt = &now
				m.UpdatedAt = now
			}
			s.mu.Unlock()
		}
		return res.RowsAffected > 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.models[id]
	if !ok || m.DeletedAt != nil {
		return false
	}
	now := time.Now().UTC()
	m.DeletedAt = &now
	m.UpdatedAt = now
	return true
}

// ========== 交易员 ==========

// CreateTrader 创建交易员。
func (s *Store) CreateTrader(t *model.Trader) {
	if t.ID == "" {
		t.ID = newID()
	}
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	if s.db != nil {
		s.db.Create(t)
		s.mu.Lock()
		s.traders[t.ID] = t
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traders[t.ID] = t
}

// GetTrader 按 ID 查交易员（返回深拷贝副本，调用方可安全修改）。
func (s *Store) GetTrader(id string) (*model.Trader, bool) {
	if s.db != nil {
		var t model.Trader
		if err := s.db.First(&t, "id = ?", id).Error; err != nil {
			return nil, false
		}
		return cloneTrader(&t), true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.traders[id]
	if !ok {
		return nil, false
	}
	return cloneTrader(t), true
}

// ListTraders 列出全部交易员（返回深拷贝副本切片）。
func (s *Store) ListTraders() []*model.Trader {
	if s.db != nil {
		var rows []model.Trader
		s.db.Order("created_at DESC").Find(&rows)
		out := make([]*model.Trader, 0, len(rows))
		for i := range rows {
			out = append(out, cloneTrader(&rows[i]))
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Trader, 0, len(s.traders))
	for _, t := range s.traders {
		out = append(out, cloneTrader(t))
	}
	return out
}

// UpdateTrader 更新交易员（入参副本锁内替换）。
func (s *Store) UpdateTrader(t *model.Trader) {
	t.UpdatedAt = time.Now().UTC()
	if s.db != nil {
		s.db.Save(t)
		s.mu.Lock()
		s.traders[t.ID] = t
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traders[t.ID] = t
}

// WithTrader 在写锁内原子执行状态迁移回调（读-改-写一体）。
// 回调入参为深拷贝副本：修改失败（返回 error）时 map 内对象不受任何影响，
// 成功时才写回。这是交易员状态机的唯一正确入口：并发 start/pause 不会互相踩踏。
// DB 路径走事务：First → mutate → Save，成功后同步内存镜像。
func (s *Store) WithTrader(id string, mutate func(*model.Trader) error) (*model.Trader, error) {
	if s.db != nil {
		var out *model.Trader
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var t model.Trader
			if err := tx.First(&t, "id = ?", id).Error; err != nil {
				return ErrNotFound
			}
			cp := cloneTrader(&t)
			if err := mutate(cp); err != nil {
				return err
			}
			cp.UpdatedAt = time.Now().UTC()
			if err := tx.Save(cp).Error; err != nil {
				return err
			}
			out = cloneTrader(cp)
			s.mu.Lock()
			s.traders[id] = cp
			s.mu.Unlock()
			return nil
		})
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.traders[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := cloneTrader(t)
	if err := mutate(cp); err != nil {
		return nil, err // 副本上的改动丢弃，map 保持原样
	}
	cp.UpdatedAt = time.Now().UTC()
	s.traders[id] = cp
	return cloneTrader(cp), nil
}

// WithTraderAndPositions 写锁内原子执行回调，同时提供持仓快照。
// 用途：平仓后回写指标需要锁内重算胜率（不能调 ListPositions——同 goroutine
// 写锁内取读锁会死锁），快照由本方法在锁内构造，无额外加锁。
// 回调同样收到深拷贝副本，error 时不留改动。
func (s *Store) WithTraderAndPositions(traderID string, mutate func(*model.Trader, []*model.Position) error) (*model.Trader, error) {
	if s.db != nil {
		var out *model.Trader
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var t model.Trader
			if err := tx.First(&t, "id = ?", traderID).Error; err != nil {
				return ErrNotFound
			}
			cp := cloneTrader(&t)
			// 持仓快照从 DB 读（含已平仓）——重启后内存镜像为空，
			// 若读内存会导致胜率/指标计算只看到当前一笔
			var pos []model.Position
			if err := tx.Where("trader_id = ?", traderID).Order("opened_at ASC").Find(&pos).Error; err != nil {
				return err
			}
			posPtrs := make([]*model.Position, 0, len(pos))
			for i := range pos {
				posPtrs = append(posPtrs, &pos[i])
			}
			if err := mutate(cp, posPtrs); err != nil {
				return err
			}
			cp.UpdatedAt = time.Now().UTC()
			if err := tx.Save(cp).Error; err != nil {
				return err
			}
			out = cloneTrader(cp)
			s.mu.Lock()
			s.traders[traderID] = cp
			s.mu.Unlock()
			return nil
		})
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.traders[traderID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := cloneTrader(t)
	pos := make([]*model.Position, 0, len(s.positions))
	for _, p := range s.positions {
		pcp := *p
		pos = append(pos, &pcp)
	}
	if err := mutate(cp, pos); err != nil {
		return nil, err
	}
	cp.UpdatedAt = time.Now().UTC()
	s.traders[traderID] = cp
	return cloneTrader(cp), nil
}

// cloneTrader 深拷贝：Schedule 指针与 Parameters RawMessage 不共享底层。
func cloneTrader(t *model.Trader) *model.Trader {
	cp := *t
	if t.Schedule != nil {
		sch := *t.Schedule
		if t.Schedule.ActiveHours != nil {
			sch.ActiveHours = append([]model.ActiveHours(nil), t.Schedule.ActiveHours...)
		}
		cp.Schedule = &sch
	}
	if t.ModelConfig.Parameters != nil {
		cp.ModelConfig.Parameters = append(json.RawMessage(nil), t.ModelConfig.Parameters...)
	}
	return &cp
}

// ========== 持仓 ==========

// AddPosition 开仓（PositionBuilder 唯一写者；OPEN 状态 + 新契约字段齐填）。
func (s *Store) AddPosition(p *model.Position) {
	s.posSeq++
	p.ID = newID()
	p.OpenedAt = time.Now().UTC()
	p.Status = model.PositionOpen
	p.EntryTime = p.OpenedAt.Unix()
	if p.Quantity == 0 {
		p.Quantity = p.Size
	}
	if p.EntryQuantity == 0 {
		p.EntryQuantity = p.Size
	}
	if s.db != nil {
		s.db.Create(p)
		s.mu.Lock()
		s.positions = append(s.positions, p)
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.positions = append(s.positions, p)
}

// GetPosition 按 ID 查持仓（返回副本）。
func (s *Store) GetPosition(id string) (*model.Position, bool) {
	if s.db != nil {
		var p model.Position
		if err := s.db.First(&p, "id = ?", id).Error; err != nil {
			return nil, false
		}
		return &p, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.positions {
		if p.ID == id {
			cp := *p
			return &cp, true
		}
	}
	return nil, false
}

// ListPositions 返回全部持仓（返回副本切片），最新在前。
func (s *Store) ListPositions() []*model.Position {
	if s.db != nil {
		var rows []model.Position
		s.db.Order("opened_at DESC").Find(&rows)
		out := make([]*model.Position, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Position, 0, len(s.positions))
	for i := len(s.positions) - 1; i >= 0; i-- {
		cp := *s.positions[i]
		out = append(out, &cp)
	}
	return out
}

// ClosePosition 平仓：写入 PnL、RealizedPnL、Status、ExitTime/ExitPrice 与 ClosedAt（锁内原子），返回更新后的副本。
func (s *Store) ClosePosition(id string, pnl float64) (*model.Position, bool) {
	if s.db != nil {
		var p model.Position
		if err := s.db.First(&p, "id = ?", id).Error; err != nil {
			return nil, false
		}
		// 条件更新：closed_at IS NULL 才生效——并发双平仓只成功一次（幂等）
		now := time.Now().UTC()
		res := s.db.Model(&model.Position{}).
			Where("id = ? AND closed_at IS NULL", id).
			Updates(map[string]any{
				// 列名遵循 GORM 命名策略（PnL → pn_l，大写拆分为下划线）
				"pn_l": pnl, "realized_pn_l": pnl, "status": model.PositionClosed,
				"exit_time": now.Unix(), "exit_price": p.MarkPrice, "closed_at": now,
			})
		if res.RowsAffected == 0 {
			// 已平仓：返回库中现状
			var cur model.Position
			if err := s.db.First(&cur, "id = ?", id).Error; err == nil {
				return &cur, false
			}
			return nil, false
		}
		p.PnL = pnl
		p.RealizedPnL = pnl
		p.Status = model.PositionClosed
		p.ExitTime = now.Unix()
		p.ExitPrice = p.MarkPrice
		p.ClosedAt = &now
		// 同步内存镜像（引擎 WithTraderAndPositions 的 pos 快照源）
		s.mu.Lock()
		for i, mp := range s.positions {
			if mp.ID == id {
				s.positions[i] = &p
				break
			}
		}
		s.mu.Unlock()
		return &p, true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.positions {
		if p.ID == id {
			if p.ClosedAt != nil {
				cp := *p
				return &cp, false // 已平仓
			}
			now := time.Now().UTC()
			p.PnL = pnl
			p.RealizedPnL = pnl
			p.Status = model.PositionClosed
			p.ExitTime = now.Unix()
			p.ExitPrice = p.MarkPrice
			p.ClosedAt = &now
			cp := *p
			return &cp, true
		}
	}
	return nil, false
}

// ListPositionsByTrader 按 trader 列持仓（返回副本切片，最新在前）。
// 仅过滤 OPEN 持仓（平仓记录走 ListPositionHistory）。
func (s *Store) ListPositionsByTrader(traderID string) []*model.Position {
	if s.db != nil {
		var rows []model.Position
		s.db.Where("trader_id = ? AND closed_at IS NULL", traderID).Order("opened_at DESC").Find(&rows)
		out := make([]*model.Position, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Position, 0, 8)
	for i := len(s.positions) - 1; i >= 0; i-- {
		p := s.positions[i]
		if p.TraderID != traderID || p.ClosedAt != nil {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	return out
}

// ListPositionHistory 平仓历史（返回副本切片，最新在前，分页）。
// symbol 可选过滤；traderID 为空则返回全部。
func (s *Store) ListPositionHistory(traderID, symbol string, offset, limit int) []*model.Position {
	if s.db != nil {
		q := s.db.Where("closed_at IS NOT NULL")
		if traderID != "" {
			q = q.Where("trader_id = ?", traderID)
		}
		if symbol != "" {
			q = q.Where("symbol = ?", symbol)
		}
		var rows []model.Position
		q.Order("closed_at DESC").Offset(offset).Limit(limit).Find(&rows)
		out := make([]*model.Position, 0, len(rows))
		for i := range rows {
			out = append(out, &rows[i])
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Position, 0, 16)
	for i := len(s.positions) - 1; i >= 0; i-- {
		p := s.positions[i]
		if p.ClosedAt == nil {
			continue
		}
		if traderID != "" && p.TraderID != traderID {
			continue
		}
		if symbol != "" && p.Symbol != symbol {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	if offset > len(out) {
		offset = len(out)
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end]
}

// CountPositionHistory 平仓历史条数。
func (s *Store) CountPositionHistory(traderID, symbol string) int {
	if s.db != nil {
		q := s.db.Model(&model.Position{}).Where("closed_at IS NOT NULL")
		if traderID != "" {
			q = q.Where("trader_id = ?", traderID)
		}
		if symbol != "" {
			q = q.Where("symbol = ?", symbol)
		}
		var n int64
		q.Count(&n)
		return int(n)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, p := range s.positions {
		if p.ClosedAt == nil {
			continue
		}
		if traderID != "" && p.TraderID != traderID {
			continue
		}
		if symbol != "" && p.Symbol != symbol {
			continue
		}
		n++
	}
	return n
}

// NewRefreshToken 生成 refresh token（32 字节 hex）。
// S2：crypto/rand 失败即 panic（fail-closed）——refresh token 是安全凭据，
// 不可退化为可预测的时间戳；crypto/rand 失败意味着熵源系统级故障，panic 比降级安全。
func NewRefreshToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// newID 生成 32 位十六进制随机 ID（同样 fail-closed）。
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

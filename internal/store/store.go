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

	"fxcore/internal/model"
)

// ErrNotFound 存储未命中。
var ErrNotFound = errors.New("store: not found")

// Store 进程内存储。
// 并发模型：读方法返回结构体副本（调用方可安全持有），
// 写方法在写锁内完成；状态迁移类操作必须走 WithTrader（锁内原子读改写）。
type Store struct {
	mu         sync.RWMutex
	users      map[string]*model.User // by user id
	emailIndex map[string]string      // email -> user id
	models     map[string]*model.AIModel
	traders    map[string]*model.Trader
	positions  []*model.Position // 追加序，最新在末尾
	posSeq     int64
}

// Config 初始化配置。
type Config struct {
	AdminEmail     string
	AdminPassword  string
	AdminSignSecret string // 为空则自动生成
}

// New 初始化存储并播种管理员账号。
func New(cfg Config) (*Store, error) {
	s := &Store{
		users:      make(map[string]*model.User),
		emailIndex: make(map[string]string),
		models:     make(map[string]*model.AIModel),
		traders:    make(map[string]*model.Trader),
		positions:  make([]*model.Position, 0, 16),
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
	s.users[u.ID] = u
	s.emailIndex[u.Email] = u.ID
	return nil
}

// ========== 用户 ==========

// GetUserByEmail 按邮箱查用户（返回副本）。
func (s *Store) GetUserByEmail(email string) (*model.User, bool) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ID == "" {
		m.ID = newID()
	}
	m.CreatedAt = time.Now().UTC()
	m.UpdatedAt = m.CreatedAt
	s.models[m.ID] = m
}

// GetModel 按 ID 查未删除模型（返回副本）。
func (s *Store) GetModel(id string) (*model.AIModel, bool) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	m.UpdatedAt = time.Now().UTC()
	s.models[m.ID] = m
}

// DeleteModel 软删除，返回是否命中。
func (s *Store) DeleteModel(id string) bool {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = newID()
	}
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	s.traders[t.ID] = t
}

// GetTrader 按 ID 查交易员（返回深拷贝副本，调用方可安全修改）。
func (s *Store) GetTrader(id string) (*model.Trader, bool) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	t.UpdatedAt = time.Now().UTC()
	s.traders[t.ID] = t
}

// WithTrader 在写锁内原子执行状态迁移回调（读-改-写一体）。
// 回调入参为深拷贝副本：修改失败（返回 error）时 map 内对象不受任何影响，
// 成功时才写回。这是交易员状态机的唯一正确入口：并发 start/pause 不会互相踩踏。
func (s *Store) WithTrader(id string, mutate func(*model.Trader) error) (*model.Trader, error) {
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

// AddPosition 开仓。
func (s *Store) AddPosition(p *model.Position) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.posSeq++
	p.ID = newID()
	p.OpenedAt = time.Now().UTC()
	s.positions = append(s.positions, p)
}

// GetPosition 按 ID 查持仓（返回副本）。
func (s *Store) GetPosition(id string) (*model.Position, bool) {
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Position, 0, len(s.positions))
	for i := len(s.positions) - 1; i >= 0; i-- {
		cp := *s.positions[i]
		out = append(out, &cp)
	}
	return out
}

// ClosePosition 平仓：写入 PnL 与 ClosedAt（锁内原子），返回更新后的副本。
func (s *Store) ClosePosition(id string, pnl float64) (*model.Position, bool) {
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
			p.ClosedAt = &now
			cp := *p
			return &cp, true
		}
	}
	return nil, false
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

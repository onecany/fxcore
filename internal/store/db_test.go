package store

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"fxcore/internal/model"
)

// TestPersistentCRUD sqlite 文件持久化：创建 → 重开 → 数据仍在（重启不丢）。
func TestPersistentCRUD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fxcore.db")

	// 第一次打开 + 播种 + 写入
	db, err := OpenDB(path, "")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	s, err := New(Config{AdminEmail: "admin@x.com", AdminPassword: "pw"}, db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// 交易所
	ex := &model.Exchange{ID: "ex1", UserID: "u1", ExchangeType: "binance", AccountName: "main", Enabled: true}
	s.CreateExchange(ex)
	// 策略（Config RawMessage 落库往返）
	s.CreateStrategy(&model.Strategy{
		ID:       "st1",
		UserID:   "u1",
		Name:     "persist-me",
		IsActive: true,
		Config:   []byte(`{"risk_control":{"max_positions":5}}`),
	})
	sqlDB, _ := db.DB()
	sqlDB.Close()

	// 重开（模拟重启）：数据必须还在，且 admin 不重复播种
	db2, err := OpenDB(path, "")
	if err != nil {
		t.Fatalf("OpenDB 2: %v", err)
	}
	s2, err := New(Config{AdminEmail: "admin@x.com", AdminPassword: "pw"}, db2)
	if err != nil {
		t.Fatalf("New 2: %v", err)
	}

	if u, ok := s2.GetUserByEmail("admin@x.com"); !ok || u.Nickname != "admin" {
		t.Fatalf("admin not reloaded: %+v", u)
	}
	if ex2, ok := s2.GetExchange("ex1"); !ok || ex2.AccountName != "main" || !ex2.Enabled {
		t.Fatalf("exchange not reloaded: %+v", ex2)
	}
	if len(s2.ListExchanges("")) != 1 {
		t.Fatalf("ListExchanges = %d, want 1", len(s2.ListExchanges("")))
	}
	st2, ok := s2.GetStrategy("st1")
	if !ok || st2.Name != "persist-me" || !st2.IsActive {
		t.Fatalf("strategy not reloaded: %+v", st2)
	}
	if act, ok := s2.GetActiveStrategy(""); !ok || act.ID != "st1" {
		t.Fatalf("active strategy not reloaded")
	}
	// 激活事务：再建一条并激活，旧激活必须被清
	s2.CreateStrategy(&model.Strategy{ID: "st2", UserID: "u1", Name: "other"})
	if _, ok := s2.ActivateStrategy("", "st2"); !ok {
		t.Fatal("activate st2 failed")
	}
	if act, _ := s2.GetActiveStrategy(""); act.ID != "st2" {
		t.Fatalf("active = %s, want st2", act.ID)
	}
	// 软删除
	if !s2.DeleteExchange("ex1") {
		t.Fatal("delete exchange failed")
	}
	if _, ok := s2.GetExchange("ex1"); ok {
		t.Fatal("exchange should be soft-deleted")
	}

	// 模型（含加密列 + 软删）
	s2.CreateModel(&model.AIModel{ID: "m1", Name: "deepseek", Provider: "deepseek", ModelName: "v4-flash", APIKeyEnc: "ENC:v1:xxx", APIKeyPrefix: "sk-****abcd"})
	if m, ok := s2.GetModel("m1"); !ok || m.Provider != "deepseek" || m.APIKeyEnc != "ENC:v1:xxx" {
		t.Fatalf("model not reloaded: %+v", m)
	}

	// 交易员（嵌套结构体 serializer:json 往返 + WithTrader 事务迁移）
	tr := &model.Trader{
		ID:       "t1",
		Name:     "trader-a",
		Exchange: "binance",
		Status:   model.StatusIdle,
		ModelConfig: model.ModelConfig{Provider: "deepseek", ModelID: "m1",
			Parameters: json.RawMessage(`{"temperature":0.7}`)},
		RiskConfig: model.RiskConfig{MaxPositionSize: 100, StopLoss: 0.02},
		Schedule:   &model.Schedule{Interval: 60, ActiveHours: []model.ActiveHours{{Start: "09:00", End: "17:00"}}},
		Metrics:    model.Metrics{TotalPnL: 12.5, WinRate: 0.6, TradeCount: 10},
	}
	s2.CreateTrader(tr)
	tr2, ok := s2.GetTrader("t1")
	if !ok {
		t.Fatal("trader not reloaded")
	}
	if tr2.ModelConfig.Provider != "deepseek" || string(tr2.ModelConfig.Parameters) != `{"temperature":0.7}` {
		t.Fatalf("model_config serializer broken: %+v", tr2.ModelConfig)
	}
	if tr2.Schedule == nil || tr2.Schedule.Interval != 60 || len(tr2.Schedule.ActiveHours) != 1 {
		t.Fatalf("schedule serializer broken: %+v", tr2.Schedule)
	}
	if tr2.Metrics.WinRate != 0.6 {
		t.Fatalf("metrics serializer broken: %+v", tr2.Metrics)
	}
	// WithTrader 事务迁移：idle → running
	tr3, err := s2.WithTrader("t1", func(t *model.Trader) error { t.Status = model.StatusRunning; return nil })
	if err != nil || tr3.Status != model.StatusRunning {
		t.Fatalf("withTrader failed: %v %+v", err, tr3)
	}
	// 回滚路径：回调报错 → DB 与内存都不变
	if _, err := s2.WithTrader("t1", func(t *model.Trader) error { return errors.New("boom") }); err == nil {
		t.Fatal("withTrader should return error")
	}
	if tr4, _ := s2.GetTrader("t1"); tr4.Status != model.StatusRunning {
		t.Fatalf("status should stay running after rollback, got %s", tr4.Status)
	}

	// Telegram 单例（Upsert 合并绑定字段）
	tg := &model.TelegramConfig{BotTokenEnc: "ENC:v1:tok", Language: "zh"}
	s2.UpsertTelegramConfig("u1", tg)
	if got, ok := s2.GetTelegramConfig("u1"); !ok || got.BotTokenEnc != "ENC:v1:tok" {
		t.Fatalf("telegram not saved: %+v", got)
	}
	tg2 := &model.TelegramConfig{BotTokenEnc: "ENC:v1:tok", ChatID: "12345", Language: "zh"}
	s2.UpsertTelegramConfig("u1", tg2)
	if got, _ := s2.GetTelegramConfig("u1"); got.ChatID != "12345" {
		t.Fatalf("telegram chat_id not merged: %+v", got)
	}
	sqlDB2, _ := db2.DB()
	sqlDB2.Close()

	// 第三次打开：全部实体仍在（重启不丢）
	db3, err := OpenDB(path, "")
	if err != nil {
		t.Fatalf("OpenDB 3: %v", err)
	}
	s3, err := New(Config{AdminEmail: "admin@x.com", AdminPassword: "pw"}, db3)
	if err != nil {
		t.Fatalf("New 3: %v", err)
	}
	if len(s3.ListTraders("")) != 1 {
		t.Fatalf("traders after reopen = %d", len(s3.ListTraders("")))
	}
	if len(s3.ListModels("")) != 1 {
		t.Fatalf("models after reopen = %d", len(s3.ListModels("")))
	}
	if tg3, ok := s3.GetTelegramConfig("u1"); !ok || tg3.ChatID != "12345" {
		t.Fatalf("telegram after reopen: %+v", tg3)
	}
	sqlDB3, _ := db3.DB()
	sqlDB3.Close()
}

// TestMySQLDSNDetect DSN 判定 + scheme 剥离。
func TestMySQLDSNDetect(t *testing.T) {
	cases := []struct{ dsn string; want bool }{
		{"", false},
		{"data/fxcore.db", false},
		{"/abs/path.db", false},
		{"mysql://user:pass@tcp(host:3306)/fxcore", true},
		{"user:pass@tcp(127.0.0.1:3306)/fxcore?charset=utf8mb4", true},
		{"user:pass@unix(/tmp/mysql.sock)/fxcore", true},
	}
	for _, c := range cases {
		if got := isMySQLDSN(c.dsn); got != c.want {
			t.Errorf("isMySQLDSN(%q) = %v, want %v", c.dsn, got, c.want)
		}
	}
	if got := stripMySQLScheme("mysql://user:pass@tcp(h:3306)/fxcore"); got != "user:pass@tcp(h:3306)/fxcore" {
		t.Errorf("stripMySQLScheme = %q", got)
	}
	if got := stripMySQLScheme("user:pass@tcp(h:3306)/fxcore"); got != "user:pass@tcp(h:3306)/fxcore" {
		t.Errorf("stripMySQLScheme noop = %q", got)
	}
}

// TestPersistentStreamData 流式运行时数据（订单/成交/持仓/决策/回测/辩论）持久化 + 幂等。
func TestPersistentStreamData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fxcore.db")
	db, err := OpenDB(path, "")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	s, _ := New(Config{AdminEmail: "a@x.com", AdminPassword: "pw"}, db)

	// 订单幂等：同 exchange_order_id 重复 AddOrder 返回既有行
	o1, created := s.AddOrder(&model.Order{TraderID: "t1", ExchangeOrderID: "EX-1", Symbol: "BTCUSDT", Side: "buy", Type: "market", Quantity: 1, Status: "NEW"})
	if !created {
		t.Fatal("first AddOrder should create")
	}
	o2, created := s.AddOrder(&model.Order{TraderID: "t1", ExchangeOrderID: "EX-1", Symbol: "BTCUSDT"})
	if created {
		t.Fatal("second AddOrder same exchange_order_id should be idempotent")
	}
	if o2.ID != o1.ID {
		t.Fatalf("idempotent order id mismatch: %s vs %s", o1.ID, o2.ID)
	}

	// 成交幂等：同 exchange_trade_id
	f1, created := s.AddFill(&model.Fill{TraderID: "t1", OrderID: o1.ID, ExchangeTradeID: "TX-1", Symbol: "BTCUSDT", Side: "buy", Price: 60000, Quantity: 1})
	if !created {
		t.Fatal("first AddFill should create")
	}
	if f2, created := s.AddFill(&model.Fill{TraderID: "t1", OrderID: o1.ID, ExchangeTradeID: "TX-1", Symbol: "BTCUSDT"}); created || f2.ID != f1.ID {
		t.Fatalf("fill idempotency broken: %v", f2)
	}

	// 持仓开平
	s.AddPosition(&model.Position{TraderID: "t1", Symbol: "BTCUSDT", Side: "long", Size: 1, EntryPrice: 60000, Quantity: 1, EntryQuantity: 1})
	open := s.ListPositionsByTrader("t1")
	if len(open) != 1 {
		t.Fatalf("open positions = %d", len(open))
	}
	if _, ok := s.ClosePosition(open[0].ID, 500); !ok {
		t.Fatal("close position failed")
	}
	if len(s.ListPositionHistory("t1", "", "", 0, 10)) != 1 {
		t.Fatal("position history = 0")
	}

	// 决策 + 权益
	s.AddDecision(&model.DecisionRecord{TraderID: "t1", CycleNumber: 1, DecisionJSON: `[{"symbol":"BTCUSDT","action":"wait"}]`, Success: true})
	s.AddDecision(&model.DecisionRecord{TraderID: "t1", CycleNumber: 2, Success: false})
	if n := s.CountDecisions("t1", ""); n != 2 {
		t.Fatalf("decisions = %d", n)
	}
	if latest, ok := s.LatestDecisionByTrader("t1"); !ok || latest.CycleNumber != 2 {
		t.Fatalf("latest decision = %+v", latest)
	}
	s.AddEquitySnapshot(&model.EquitySnapshot{TraderID: "t1", TotalEquity: 10000})
	s.AddEquitySnapshot(&model.EquitySnapshot{TraderID: "t1", TotalEquity: 10200})
	if eq := s.ListEquityByTrader("t1", ""); len(eq) != 2 || eq[1].TotalEquity != 10200 {
		t.Fatalf("equity snapshots = %+v", eq)
	}

	// 回测全生命周期
	s.CreateBacktestRun(&model.BacktestRun{RunID: "bt-1", UserID: "u1", State: "running", SymbolCount: 1})
	s.AddBacktestEquity(&model.BacktestEquity{RunID: "bt-1", Timestamp: 1, Equity: 1000})
	s.AddBacktestTrade(&model.BacktestTrade{RunID: "bt-1", Timestamp: 2, Symbol: "BTCUSDT", Action: "open_long", Quantity: 1, Price: 60000})
	s.AddBacktestDecision(&model.BacktestDecision{RunID: "bt-1", Cycle: 1, Payload: json.RawMessage(`{"a":1}`)})
	s.SaveBacktestCheckpoint("bt-1", json.RawMessage(`{"pos":0}`))
	if runs := s.ListBacktestRuns(""); len(runs) != 1 || runs[0].State != "running" {
		t.Fatalf("backtest runs = %+v", runs)
	}
	if eq := s.ListBacktestEquities("bt-1"); len(eq) != 1 {
		t.Fatal("backtest equities = 0")
	}
	if d, ok := s.GetBacktestDecision("bt-1", 1); !ok || string(d.Payload) != `{"a":1}` {
		t.Fatalf("backtest decision = %+v", d)
	}
	if cp, ok := s.GetBacktestCheckpoint("bt-1"); !ok || string(cp.Payload) != `{"pos":0}` {
		t.Fatalf("checkpoint = %+v", cp)
	}

	// 辩论会话 + 消息
	s.CreateDebateSession(&model.DebateSession{ID: "d1", Name: "debate-1", Status: "running", Symbol: "BTCUSDT", MaxRounds: 3, CurrentRound: 1})
	s.AddDebateParticipant(&model.DebateParticipant{SessionID: "d1", AIModelID: "m1", Personality: "bull"})
	s.AddDebateMessage(&model.DebateMessage{SessionID: "d1", ParticipantID: "p1", Round: 1, Content: "看多", Personality: "bull"})
	s.AddDebateVote(&model.DebateVote{SessionID: "d1", AIModelID: "m1", Action: "open_long", Symbol: "BTCUSDT", Confidence: 0.8})
	if sess, ok := s.GetDebateSession("d1", ""); !ok || sess.CurrentRound != 1 {
		t.Fatalf("debate session = %+v", sess)
	}
	if msgs := s.ListDebateMessages("d1"); len(msgs) != 1 {
		t.Fatal("debate messages = 0")
	}

	// 重开验证全部在
	sqlDB, _ := db.DB()
	sqlDB.Close()
	db2, err := OpenDB(path, "")
	if err != nil {
		t.Fatalf("OpenDB 2: %v", err)
	}
	s2, _ := New(Config{AdminEmail: "a@x.com", AdminPassword: "pw"}, db2)
	if len(s2.ListOrders("t1", "")) != 1 || len(s2.ListFillsByTrader("t1", "")) != 1 {
		t.Fatal("orders/fills lost after reopen")
	}
	if len(s2.ListPositionHistory("t1", "", "", 0, 10)) != 1 {
		t.Fatal("position history lost after reopen")
	}
	if s2.CountDecisions("t1", "") != 2 || len(s2.ListEquityByTrader("t1", "")) != 2 {
		t.Fatal("decisions/equities lost after reopen")
	}
	if len(s2.ListBacktestRuns("")) != 1 || len(s2.ListBacktestEquities("bt-1")) != 1 || len(s2.ListBacktestTrades("bt-1")) != 1 || len(s2.ListBacktestDecisions("bt-1")) != 1 {
		t.Fatal("backtest data lost after reopen")
	}
	if _, ok := s2.GetBacktestCheckpoint("bt-1"); !ok {
		t.Fatal("checkpoint lost after reopen")
	}
	if len(s2.ListDebateSessions("")) != 1 || len(s2.ListDebateMessages("d1")) != 1 || len(s2.ListDebateVotes("d1")) != 1 || len(s2.ListDebateParticipants("d1")) != 1 {
		t.Fatal("debate data lost after reopen")
	}
	// 级联删除
	if !s2.DeleteBacktestRun("bt-1") {
		t.Fatal("delete backtest run failed")
	}
	if len(s2.ListBacktestEquities("bt-1")) != 0 || len(s2.ListBacktestTrades("bt-1")) != 0 {
		t.Fatal("cascade delete failed")
	}
	if !s2.DeleteDebateSession("d1") {
		t.Fatal("delete debate session failed")
	}
	if len(s2.ListDebateMessages("d1")) != 0 {
		t.Fatal("debate cascade delete failed")
	}
	sqlDB2, _ := db2.DB()
	sqlDB2.Close()
}

// TestPersistentSeedIdempotent 重复 New 不重复播种（admin 只一条）。
func TestPersistentSeedIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fxcore.db")
	db, err := OpenDB(path, "")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.DB() // 保持打开到测试结束

	s1, _ := New(Config{AdminEmail: "a@x.com", AdminPassword: "pw"}, db)
	s2, _ := New(Config{AdminEmail: "a@x.com", AdminPassword: "pw"}, db)
	_ = s1
	var count int64
	db.Model(&model.User{}).Where("email = ?", "a@x.com").Count(&count)
	if count != 1 {
		t.Fatalf("admin count = %d, want 1", count)
	}
	_ = s2
	_ = time.Now
}

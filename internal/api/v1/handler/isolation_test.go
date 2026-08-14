package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/debate"
	"fxcore/internal/llm"
	"fxcore/internal/model"
	"fxcore/internal/store"
)

// ========== P1 多用户隔离断言矩阵 ==========
// 每个"越权返 404"的端点一条用例：资源属主 user-other，请求方 single-user（无 claims 回退）。

// backtest：他人 run 的所有 run 级端点 → 404。
// 通过直接调用 engine 内部创建他人 run（store 直灌），再验证 handler 全部拒绝。
func TestIsolationBacktestRunForeign(t *testing.T) {
	h, eng := newBacktestHandler(t)
	// 用 engine.Start 以 user-other 身份起一个 run，拿到 runID
	now := time.Now()
	meta, apiErr := eng.Start("user-other", dto.BacktestConfig{
		Symbols:       []string{"BTC-USDT"},
		ReplayOnly:    true,
		StartTime:     now.Add(-3600 * time.Second).Unix(),
		EndTime:       now.Unix(),
		InitialBalance: 10000,
	})
	if apiErr != nil {
		t.Fatalf("start other run: %v", apiErr)
	}
	runID := meta.RunID
	t.Cleanup(func() {
		_, _ = eng.Stop(runID)
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/backtest/status", h.Status)
	r.GET("/backtest/equity", h.Equity)
	r.GET("/backtest/trades", h.Trades)
	r.GET("/backtest/metrics", h.Metrics)
	r.GET("/backtest/trace", h.Trace)
	r.GET("/backtest/decisions", h.Decisions)
	r.GET("/backtest/export", h.Export)
	r.GET("/backtest/klines", h.Klines)
	r.POST("/backtest/:action", h.Control)

	// 越权访问他人 run：全部 404
	for _, path := range []string{
		"/backtest/status?run_id=" + runID,
		"/backtest/equity?run_id=" + runID,
		"/backtest/trades?run_id=" + runID,
		"/backtest/metrics?run_id=" + runID,
		"/backtest/trace?run_id=" + runID,
		"/backtest/decisions?run_id=" + runID,
		"/backtest/export?run_id=" + runID,
		"/backtest/klines?run_id=" + runID,
	} {
		w := doJSON(r, http.MethodGet, path, "")
		if w.Code != http.StatusNotFound {
			t.Fatalf("GET %s want 404, got %d %s", path, w.Code, w.Body.String())
		}
	}
	// 控制端点（pause）同样 404
	w := doJSON(r, http.MethodPost, "/backtest/pause", `{"run_id":"`+runID+`"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("pause foreign run want 404, got %d %s", w.Code, w.Body.String())
	}
}

// debate：他人会话 Get/Execute/Delete/Messages/Votes/Stream → 404。
func TestIsolationDebateForeign(t *testing.T) {
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	models := &debateFakeModels{models: map[string]*llm.Model{
		"m1": {Provider: "gpt", ModelName: "gpt-4o", APIKey: "sk-test"},
		"m2": {Provider: "gpt", ModelName: "gpt-4o", APIKey: "sk-test"},
	}}
	ai := llm.NewWithTransport(&debateFakeAI{body: `{"choices":[{"message":{"content":"test"}}]}`}, 5*time.Second)
	eng := debate.NewEngine(st, models, ai)
	h := NewDebateHandler(eng)

	// 以 user-other 创建会话
	sess, apiErr := eng.Create("user-other", dto.CreateDebateRequest{
		Name: "foreign", StrategyID: "s1", Symbol: "BTC-USDT",
		Participants: []string{"m1", "m2"}, MaxRounds: 1,
	})
	if apiErr != nil {
		t.Fatalf("create foreign debate: %v", apiErr)
	}
	id := sess.ID

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/debates/:id", h.Get)
	r.GET("/debates/:id/messages", h.Messages)
	r.GET("/debates/:id/votes", h.Votes)
	r.GET("/debates/:id/stream", h.Stream)
	r.POST("/debates/:id/execute", h.Execute)
	r.DELETE("/debates/:id", h.Delete)

	// 全部越权 404
	cases := []struct{ method, path string }{
		{"GET", "/debates/" + id},
		{"GET", "/debates/" + id + "/messages"},
		{"GET", "/debates/" + id + "/votes"},
		{"GET", "/debates/" + id + "/stream"},
		{"POST", "/debates/" + id + "/execute"},
		{"DELETE", "/debates/" + id},
	}
	for _, tc := range cases {
		w := doJSON(r, tc.method, tc.path, `{"trader_id":"t1"}`)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s %s want 404, got %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

// strategy：他人策略 Get/Update/Delete/Activate/Duplicate → 404。
func TestIsolationStrategyForeign(t *testing.T) {
	h, st, _ := strategyTestEnv(t, `{"choices":[{"message":{"content":"x"}}]}`)
	st.CreateStrategy(&model.Strategy{ID: "s-other", UserID: "user-other", Name: "theirs", Config: []byte(`{}`)})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/strategies/:id", h.Get)
	r.PUT("/strategies/:id", h.Update)
	r.DELETE("/strategies/:id", h.Delete)
	r.POST("/strategies/:id/activate", h.Activate)
	r.POST("/strategies/:id/duplicate", h.Duplicate)

	cases := []struct{ method, path, body string }{
		{"GET", "/strategies/s-other", ""},
		{"PUT", "/strategies/s-other", `{"config":{}}`},
		{"DELETE", "/strategies/s-other", ""},
		{"POST", "/strategies/s-other/activate", ""},
		{"POST", "/strategies/s-other/duplicate", ""},
	}
	for _, tc := range cases {
		w := doJSON(r, tc.method, tc.path, tc.body)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s %s want 404, got %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

// telegram：模型引用校验——Save 与 SetModel 引用他人模型都 → 404（P2 收尾的模型引用归属校验）。
func TestIsolationTelegramForeignModel(t *testing.T) {
	h, st := newTelegramHandler(t)
	st.CreateModel(&model.AIModel{ID: "m-other", UserID: "user-other", Provider: "deepseek", ModelName: "deepseek-chat", APIKeyEnc: "enc"})
	r := telegramRouter(h)

	// Save 引用他人模型 → 404（保存时即校验模型引用）
	w := doJSON(r, http.MethodPost, "/telegram", `{"bot_token":"123456:ABC","model_id":"m-other"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("save with foreign model want 404, got %d %s", w.Code, w.Body.String())
	}
	// 先用自己的模型保存成功，再 SetModel 到他人模型 → 404
	st.CreateModel(&model.AIModel{ID: "m1", UserID: "single-user", Provider: "deepseek", ModelName: "deepseek-chat", APIKeyEnc: "enc"})
	w = doJSON(r, http.MethodPost, "/telegram", `{"bot_token":"123456:ABC","model_id":"m1"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("save own model want 200, got %d %s", w.Code, w.Body.String())
	}
	w = doJSON(r, http.MethodPost, "/telegram/model", `{"model_id":"m-other"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("set model foreign want 404, got %d %s", w.Code, w.Body.String())
	}
}

// positions：ListPositions 只返回本用户 trader 的持仓（聚合过滤）。
func TestIsolationPositionsFiltered(t *testing.T) {
	h, st := newTraderHandler(t)
	// 两个用户各一个 trader + 持仓
	for _, uid := range []string{"single-user", "user-other"} {
		st.CreateTrader(&model.Trader{
			ID: "t-" + uid, UserID: uid, Name: "t", Exchange: "binance",
			ModelConfig: model.ModelConfig{Provider: "deepseek", ModelID: "m1"},
			Status:      model.StatusIdle,
		})
		st.AddPosition(&model.Position{
			TraderID: "t-" + uid, Symbol: "BTC-USDT", Side: "long",
			Size: 0.01, EntryPrice: 60000,
		})
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/positions", h.ListPositions)

	w := doJSON(r, http.MethodGet, "/positions", "")
	if w.Code != http.StatusOK {
		t.Fatalf("positions want 200, got %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data []dto.PositionDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope parse: %v", err)
	}
	for _, p := range env.Data {
		if p.TraderID == "t-user-other" {
			t.Fatalf("positions leaked foreign trader data: %s", w.Body.String())
		}
	}
	if len(env.Data) == 0 {
		t.Fatalf("want own positions visible, got empty")
	}
}

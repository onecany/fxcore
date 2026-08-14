package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/model"
	"fxcore/internal/store"
)

// newTraderHandler 组装 trader handler（内存 store + 引擎 nil）。
func newTraderHandler(t *testing.T) (*TraderHandler, *store.Store) {
	t.Helper()
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 创建所需引用：模型 m1 + 策略 s1（都归 single-user，与无 claims 的 currentUserID 对齐）
	st.CreateModel(&model.AIModel{ID: "m1", UserID: "single-user", Provider: "deepseek", ModelName: "deepseek-chat", Status: "active"})
	st.CreateStrategy(&model.Strategy{ID: "s1", UserID: "single-user", Name: "s1"})
	h := NewTraderHandler(service.NewTraderService(st, nil))
	return h, st
}

func traderRouter(h *TraderHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/traders", h.Create)
	r.GET("/traders/:id", h.Get)
	r.PATCH("/traders/:id", h.Update)
	r.POST("/traders/:id/start", h.Start)
	r.POST("/traders/:id/pause", h.Pause)
	r.POST("/traders/:id/resume", h.Resume)
	r.POST("/traders/:id/stop", h.Stop)
	r.GET("/positions", h.ListPositions)
	r.DELETE("/positions/:id", h.ClosePosition)
	return r
}

func createTraderReq() string {
	return `{"name":"t1","exchange":"binance","model_config":{"provider":"deepseek","model_id":"m1"},"strategy_id":"s1","risk_config":{"max_position_size":100}}`
}

func createTrader(t *testing.T, r *gin.Engine) string {
	t.Helper()
	w := doJSON(r, http.MethodPost, "/traders", createTraderReq())
	if w.Code != http.StatusOK {
		t.Fatalf("create trader failed: %d %s", w.Code, w.Body.String())
	}
	_, data := parseEnvelope(t, w)
	var tr dto.TraderDTO
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("create resp parse: %v", err)
	}
	if tr.ID == "" {
		t.Fatalf("create returned empty id")
	}
	if tr.Status != "idle" {
		t.Fatalf("new trader must be idle, got %q", tr.Status)
	}
	return tr.ID
}

// 创建：引用不存在的模型 → 400 + 1001。
func TestTraderCreateModelNotFound(t *testing.T) {
	h, _ := newTraderHandler(t)
	r := traderRouter(h)
	body := `{"name":"t1","exchange":"binance","model_config":{"provider":"deepseek","model_id":"ghost"},"strategy_id":"s1","risk_config":{"max_position_size":100}}`
	w := doJSON(r, http.MethodPost, "/traders", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1001 {
		t.Fatalf("want 1001, got %d", code)
	}
}

// 创建：非法交易所 → 400 + 1001。
func TestTraderCreateUnsupportedExchange(t *testing.T) {
	h, _ := newTraderHandler(t)
	r := traderRouter(h)
	body := `{"name":"t1","exchange":"nope","model_config":{"provider":"deepseek","model_id":"m1"},"strategy_id":"s1","risk_config":{"max_position_size":100}}`
	w := doJSON(r, http.MethodPost, "/traders", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", w.Code, w.Body.String())
	}
}

// 状态机全迁移：idle→start→running→pause→paused→resume→running→stop→stopped→restart→running。
func TestTraderStateMachineLifecycle(t *testing.T) {
	h, _ := newTraderHandler(t)
	r := traderRouter(h)
	id := createTrader(t, r)

	// start: idle→running
	w := doJSON(r, http.MethodPost, "/traders/"+id+"/start", ``)
	if w.Code != http.StatusOK {
		t.Fatalf("start want 200, got %d %s", w.Code, w.Body.String())
	}
	// pause: running→paused
	w = doJSON(r, http.MethodPost, "/traders/"+id+"/pause", ``)
	if w.Code != http.StatusOK {
		t.Fatalf("pause want 200, got %d %s", w.Code, w.Body.String())
	}
	// resume: paused→running
	w = doJSON(r, http.MethodPost, "/traders/"+id+"/resume", ``)
	if w.Code != http.StatusOK {
		t.Fatalf("resume want 200, got %d %s", w.Code, w.Body.String())
	}
	// stop: running→stopped
	w = doJSON(r, http.MethodPost, "/traders/"+id+"/stop", ``)
	if w.Code != http.StatusOK {
		t.Fatalf("stop want 200, got %d %s", w.Code, w.Body.String())
	}
	// restart: stopped→running（stopped 可重启）
	w = doJSON(r, http.MethodPost, "/traders/"+id+"/start", ``)
	if w.Code != http.StatusOK {
		t.Fatalf("restart want 200, got %d %s", w.Code, w.Body.String())
	}
}

// 重复 start（已 running）→ 1204。
func TestTraderStartTwiceConflict(t *testing.T) {
	h, _ := newTraderHandler(t)
	r := traderRouter(h)
	id := createTrader(t, r)

	doJSON(r, http.MethodPost, "/traders/"+id+"/start", ``)
	w := doJSON(r, http.MethodPost, "/traders/"+id+"/start", ``)
	if w.Code != http.StatusConflict {
		t.Fatalf("double start want 409, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1204 {
		t.Fatalf("want 1204, got %d", code)
	}
}

// 非法迁移：idle 直接 pause → 1004。
func TestTraderPauseFromIdleInvalid(t *testing.T) {
	h, _ := newTraderHandler(t)
	r := traderRouter(h)
	id := createTrader(t, r)

	w := doJSON(r, http.MethodPost, "/traders/"+id+"/pause", ``)
	if w.Code != http.StatusNotFound {
		t.Fatalf("pause from idle want 404, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1004 {
		t.Fatalf("want 1004, got %d", code)
	}
}

// 交易员不存在 → 1004。
func TestTraderNotFound(t *testing.T) {
	h, _ := newTraderHandler(t)
	r := traderRouter(h)

	w := doJSON(r, http.MethodGet, "/traders/ghost", ``)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1004 {
		t.Fatalf("want 1004, got %d", code)
	}
}

// 越权访问他人交易员：Get/Start → 404（归属校验）。
func TestTraderOwnershipForbidden(t *testing.T) {
	h, st := newTraderHandler(t)
	// 造一个属于他人（user-other）的交易员
	st.CreateTrader(&model.Trader{
		ID:          "t-other",
		UserID:      "user-other",
		Name:        "other",
		Exchange:    "binance",
		ModelConfig: model.ModelConfig{Provider: "deepseek", ModelID: "m1"},
		StrategyID:  "s1",
		RiskConfig:  model.RiskConfig{MaxPositionSize: 100},
		Status:      model.StatusIdle,
	})
	r := traderRouter(h)

	w := doJSON(r, http.MethodGet, "/traders/t-other", ``)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign get want 404, got %d %s", w.Code, w.Body.String())
	}
	w = doJSON(r, http.MethodPost, "/traders/t-other/start", ``)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign start want 404, got %d %s", w.Code, w.Body.String())
	}
}

// 平仓：需先造 trader + 持仓。持仓归属通过 TraderID → trader.UserID 校验。
// 注意：AddPosition 会强制覆盖 ID（newID()），测试用返回的实际 ID 操作。
func seedOpenPosition(t *testing.T, st *store.Store, traderID, userID string) string {
	t.Helper()
	st.CreateTrader(&model.Trader{
		ID:          traderID,
		UserID:      userID,
		Name:        "t-" + traderID,
		Exchange:    "binance",
		ModelConfig: model.ModelConfig{Provider: "deepseek", ModelID: "m1"},
		StrategyID:  "s1",
		RiskConfig:  model.RiskConfig{MaxPositionSize: 100},
		Status:      model.StatusIdle,
	})
	st.AddPosition(&model.Position{
		TraderID:   traderID,
		Symbol:     "BTC-USDT",
		Side:       "long",
		Size:       0.01,
		EntryPrice: 60000,
	})
	// AddPosition 覆盖 ID，从 store 读回实际 ID
	positions := st.ListPositions()
	for _, p := range positions {
		if p.TraderID == traderID {
			return p.ID
		}
	}
	t.Fatal("position not found after AddPosition")
	return ""
}

// 平仓成功路径：返回 closed + 归属校验通过。
func TestTraderClosePositionSuccess(t *testing.T) {
	h, st := newTraderHandler(t)
	posID := seedOpenPosition(t, st, "t1", "single-user")
	r := traderRouter(h)

	w := doJSON(r, http.MethodDelete, "/positions/"+posID, `{"pnl":123.45}`)
	if w.Code != http.StatusOK {
		t.Fatalf("close want 200, got %d %s", w.Code, w.Body.String())
	}
	_, data := parseEnvelope(t, w)
	var p dto.PositionDTO
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("close resp parse: %v", err)
	}
	if p.Status != "CLOSED" {
		t.Fatalf("want CLOSED, got %q", p.Status)
	}
}

// 平仓：pnl 为 NaN → 400 + 1001（L2 拒绝非有限值污染指标）。
func TestTraderClosePositionRejectsNaN(t *testing.T) {
	h, st := newTraderHandler(t)
	posID := seedOpenPosition(t, st, "t1", "single-user")
	r := traderRouter(h)

	w := doJSON(r, http.MethodDelete, "/positions/"+posID, `{"pnl":NaN}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("NaN close want 400, got %d %s", w.Code, w.Body.String())
	}
}

// 越权平仓他人持仓（trader 属主不匹配）→ 404。
func TestTraderClosePositionOwnership(t *testing.T) {
	h, st := newTraderHandler(t)
	posID := seedOpenPosition(t, st, "t-other", "user-other")
	r := traderRouter(h)

	w := doJSON(r, http.MethodDelete, "/positions/"+posID, `{"pnl":1.0}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign close want 404, got %d %s", w.Code, w.Body.String())
	}
}

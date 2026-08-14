package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/model"
	"fxcore/internal/pkg/jwt"
	"fxcore/internal/store"
)

// newDataHandler 组装 data handler（内存 store + klines provider nil——测试只走归属校验/空数据路径）。
func newDataHandler(t *testing.T) (*DataHandler, *store.Store) {
	t.Helper()
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 交易员 t1 归 single-user（与无 claims 的 currentUserID 对齐）；t-other 归他人
	st.CreateTrader(&model.Trader{
		ID:          "t1",
		UserID:      "single-user",
		Name:        "mine",
		Exchange:    "binance",
		ModelConfig: model.ModelConfig{Provider: "deepseek", ModelID: "m1"},
		StrategyID:  "s1",
		Status:      model.StatusIdle,
	})
	st.CreateTrader(&model.Trader{
		ID:          "t-other",
		UserID:      "user-other",
		Name:        "theirs",
		Exchange:    "binance",
		ModelConfig: model.ModelConfig{Provider: "deepseek", ModelID: "m1"},
		StrategyID:  "s1",
		Status:      model.StatusIdle,
	})
	h := NewDataHandler(service.NewDataService(st, nil), st)
	return h, st
}

func dataRouter(h *DataHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/status", h.Status)
	r.GET("/decisions", h.Decisions)
	r.GET("/trades", h.Trades)
	r.GET("/orders", h.Orders)
	r.GET("/orders/:id/fills", h.OrderFills)
	r.GET("/positions/history", h.PositionHistory)
	r.GET("/equity-history", h.EquityHistory)
	r.POST("/equity-history-batch", h.EquityHistoryBatch)
	r.GET("/klines", h.Klines)
	return r
}

// 归属校验矩阵：自己的 trader 放行（200/空数据），他人 trader 404，空 trader 放行。
// 覆盖全部 authorizeTrader 前置端点——这是多用户隔离第四轮修复的核心防线。
func TestDataOwnershipMatrix(t *testing.T) {
	h, _ := newDataHandler(t)
	r := dataRouter(h)

	cases := []struct {
		name string
		method, path string
		ownExpected int // 自己的 trader 期望码
		foreignExpected int // 他人 trader 期望码
	}{
		{"status", "GET", "/status?trader_id=", 200, 404},
		{"decisions", "GET", "/decisions?trader_id=", 200, 404},
		{"trades", "GET", "/trades?trader_id=", 200, 404},
		{"orders", "GET", "/orders?trader_id=", 200, 404},
		{"positions-history", "GET", "/positions/history?trader_id=", 200, 404},
		{"equity-history", "GET", "/equity-history?trader_id=", 200, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name+"-own", func(t *testing.T) {
			w := doJSON(r, tc.method, tc.path+"t1", "")
			if w.Code != tc.ownExpected {
				t.Fatalf("own want %d, got %d %s", tc.ownExpected, w.Code, w.Body.String())
			}
		})
		t.Run(tc.name+"-foreign", func(t *testing.T) {
			w := doJSON(r, tc.method, tc.path+"t-other", "")
			if w.Code != tc.foreignExpected {
				t.Fatalf("foreign want %d, got %d %s", tc.foreignExpected, w.Code, w.Body.String())
			}
		})
		t.Run(tc.name+"-empty", func(t *testing.T) {
			w := doJSON(r, tc.method, tc.path, "")
			if w.Code != 200 {
				t.Fatalf("empty trader want 200, got %d %s", w.Code, w.Body.String())
			}
		})
	}
}

// equity-history-batch：列表含他人 trader → 整体 404（逐个校验，先到先拒）。
func TestDataEquityBatchForeignRejected(t *testing.T) {
	h, _ := newDataHandler(t)
	r := dataRouter(h)

	w := doJSON(r, http.MethodPost, "/equity-history-batch", `{"trader_ids":["t1","t-other"]}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("batch with foreign want 404, got %d %s", w.Code, w.Body.String())
	}
}

// equity-history-batch：全自己的 trader → 200。
func TestDataEquityBatchOwnOK(t *testing.T) {
	h, _ := newDataHandler(t)
	r := dataRouter(h)

	w := doJSON(r, http.MethodPost, "/equity-history-batch", `{"trader_ids":["t1"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("batch own want 200, got %d %s", w.Code, w.Body.String())
	}
}

// OrderFills：订单属于他人 trader → 404（order → trader → user 链）。
func TestDataOrderFillsForeign(t *testing.T) {
	h, st := newDataHandler(t)
	st.AddOrder(&model.Order{ID: "o-foreign", TraderID: "t-other", Symbol: "BTC-USDT", Side: "buy", Type: "market", Status: "NEW"})
	r := dataRouter(h)

	w := doJSON(r, http.MethodGet, "/orders/o-foreign/fills", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign order fills want 404, got %d %s", w.Code, w.Body.String())
	}
}

// Klines 缺 symbol → 400 + 1001（骨架端点契约）。
func TestDataKlinesMissingSymbol(t *testing.T) {
	h, _ := newDataHandler(t)
	r := dataRouter(h)

	w := doJSON(r, http.MethodGet, "/klines", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing symbol want 400, got %d %s", w.Code, w.Body.String())
	}
}

// Decisions 分页信封结构：自己的 trader 返回 items 数组（空数据也应有信封）。
func TestDataDecisionsEnvelope(t *testing.T) {
	h, _ := newDataHandler(t)
	r := dataRouter(h)

	w := doJSON(r, http.MethodGet, "/decisions?trader_id=t1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("decisions want 200, got %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			Items      []any `json:"items"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope parse: %v body=%s", err, w.Body.String())
	}
	if env.Data.Items == nil {
		t.Fatalf("items must be [] not null: %s", w.Body.String())
	}
}

// ========== dashboard ==========

// newDashboardHandler 组装 dashboard handler。
func newDashboardHandler(t *testing.T) (*DashboardHandler, *store.Store) {
	t.Helper()
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jm, err := jwt.NewManager("test-secret-123456", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return NewDashboardHandler(st, jm), st
}

// dashboard 聚合：无交易员时仍 200 + 空聚合（不炸）。
func TestDashboardEmptyAggregate(t *testing.T) {
	h, _ := newDashboardHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/dashboard", h.GetDashboard)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("empty dashboard want 200, got %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope parse: %v", err)
	}
	if env.Code != 0 {
		t.Fatalf("want code 0, got %d", env.Code)
	}
}

// dashboard 聚合：仅返回本用户交易员（t1 归 single-user，t-other 归他人）。
// 无 claims 时 currentUserID 回退 single-user → 活跃交易员只含 t1。
func TestDashboardScopesToOwnTraders(t *testing.T) {
	h, st := newDashboardHandler(t)
	st.CreateTrader(&model.Trader{
		ID: "t1", UserID: "single-user", Name: "mine", Exchange: "binance",
		ModelConfig: model.ModelConfig{Provider: "deepseek", ModelID: "m1"}, Status: model.StatusRunning,
	})
	st.CreateTrader(&model.Trader{
		ID: "t-other", UserID: "user-other", Name: "theirs", Exchange: "binance",
		ModelConfig: model.ModelConfig{Provider: "deepseek", ModelID: "m1"}, Status: model.StatusRunning,
	})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/dashboard", h.GetDashboard)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("dashboard want 200, got %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			ActiveTraders []dto.TraderDTO `json:"active_traders"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope parse: %v body=%s", err, w.Body.String())
	}
	for _, tr := range env.Data.ActiveTraders {
		if tr.ID == "t-other" {
			t.Fatalf("dashboard leaked foreign trader: %s", w.Body.String())
		}
	}
}

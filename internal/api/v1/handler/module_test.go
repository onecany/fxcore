package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/backtest"
	"fxcore/internal/llm"
	"fxcore/internal/model"
	"fxcore/internal/pkg/crypto"
	"fxcore/internal/store"
)

// ========== model ==========

func newModelHandler(t *testing.T) (*ModelHandler, *store.Store) {
	t.Helper()
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	km, err := crypto.NewKeyManager("")
	if err != nil {
		t.Fatal(err)
	}
	return NewModelHandler(service.NewModelService(st, km)), st
}

func modelRouter(h *ModelHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/models", h.List)
	r.GET("/models/providers", h.Providers)
	r.POST("/models", h.Create)
	r.PUT("/models/:id", h.Update)
	r.DELETE("/models/:id", h.Delete)
	return r
}

// 创建模型：api_key 加密落库，响应不含明文 key。
func TestModelCreateNoKeyLeak(t *testing.T) {
	h, _ := newModelHandler(t)
	r := modelRouter(h)
	w := doJSON(r, http.MethodPost, "/models", `{"name":"m","provider":"deepseek","model_name":"deepseek-chat","api_key":"sk-super-secret-123456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("create want 200, got %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if contains(body, "sk-super-secret") {
		t.Fatalf("api key leaked in response: %s", body)
	}
	_, data := parseEnvelope(t, w)
	var m dto.AIModelDTO
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("resp parse: %v", err)
	}
	if m.ID == "" || m.APIKeyPrefix == "" {
		t.Fatalf("want id + masked prefix, got %+v", m)
	}
}

// 创建缺 api_key → 400 + 1001。
func TestModelCreateMissingKey(t *testing.T) {
	h, _ := newModelHandler(t)
	r := modelRouter(h)
	w := doJSON(r, http.MethodPost, "/models", `{"name":"m","provider":"deepseek","model_name":"deepseek-chat"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing key want 400, got %d %s", w.Code, w.Body.String())
	}
}

// 越权操作他人模型：Update/Delete → 404。
func TestModelOwnershipForbidden(t *testing.T) {
	h, st := newModelHandler(t)
	st.CreateModel(&model.AIModel{ID: "m-other", UserID: "user-other", Provider: "deepseek", ModelName: "deepseek-chat", APIKeyEnc: "enc"})
	r := modelRouter(h)

	w := doJSON(r, http.MethodPut, "/models/m-other", `{"name":"x","provider":"deepseek","model_name":"deepseek-chat"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign update want 404, got %d %s", w.Code, w.Body.String())
	}
	w = doJSON(r, http.MethodDelete, "/models/m-other", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign delete want 404, got %d %s", w.Code, w.Body.String())
	}
}

// ========== exchange ==========

func newExchangeHandler(t *testing.T) (*ExchangeHandler, *store.Store) {
	t.Helper()
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	km, err := crypto.NewKeyManager("")
	if err != nil {
		t.Fatal(err)
	}
	return NewExchangeHandler(service.NewExchangeService(st, km)), st
}

func exchangeRouter(h *ExchangeHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/exchanges", h.List)
	r.POST("/exchanges", h.Create)
	r.PUT("/exchanges/:id", h.Update)
	r.DELETE("/exchanges/:id", h.Delete)
	return r
}

// 创建交易所：凭据零泄漏 + api_key_prefix 脱敏。
func TestExchangeCreateNoLeak(t *testing.T) {
	h, _ := newExchangeHandler(t)
	r := exchangeRouter(h)
	w := doJSON(r, http.MethodPost, "/exchanges", `{"exchange_type":"binance","account_name":"main","api_key":"ak-1234567890abcdef","secret_key":"sk-abcdef1234567890"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("create want 200, got %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if contains(body, "ak-1234567890abcdef") || contains(body, "sk-abcdef1234567890") {
		t.Fatalf("credentials leaked: %s", body)
	}
	_, data := parseEnvelope(t, w)
	var e dto.ExchangeDTO
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatalf("resp parse: %v", err)
	}
	if e.APIKeyPrefix == "" {
		t.Fatalf("want masked prefix, got %+v", e)
	}
}

// 必填凭据缺失：gate 不需要 passphrase 但缺 api_key → 1001。
func TestExchangeCreateMissingCreds(t *testing.T) {
	h, _ := newExchangeHandler(t)
	r := exchangeRouter(h)
	w := doJSON(r, http.MethodPost, "/exchanges", `{"exchange_type":"binance","account_name":"main"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing creds want 400, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1001 {
		t.Fatalf("want 1001, got %d", code)
	}
}

// 越权更新他人交易所 → 404。
func TestExchangeOwnershipForbidden(t *testing.T) {
	h, st := newExchangeHandler(t)
	st.CreateExchange(&model.Exchange{ID: "e-other", UserID: "user-other", ExchangeType: "binance", AccountName: "theirs", APIKeyEnc: "enc"})
	r := exchangeRouter(h)

	w := doJSON(r, http.MethodPut, "/exchanges/e-other", `{"enabled":true}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign update want 404, got %d %s", w.Code, w.Body.String())
	}
}

// ========== telegram ==========

func newTelegramHandler(t *testing.T) (*TelegramHandler, *store.Store) {
	t.Helper()
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	km, err := crypto.NewKeyManager("")
	if err != nil {
		t.Fatal(err)
	}
	return NewTelegramHandler(service.NewTelegramService(st, km)), st
}

func telegramRouter(h *TelegramHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/telegram", h.Get)
	r.POST("/telegram", h.Save)
	r.POST("/telegram/model", h.SetModel)
	r.DELETE("/telegram/binding", h.DeleteBinding)
	return r
}

// 保存配置：成功 + 未配置时 Get → 404。
func TestTelegramSaveAndGet(t *testing.T) {
	h, st := newTelegramHandler(t)
	st.CreateModel(&model.AIModel{ID: "m1", UserID: "single-user", Provider: "deepseek", ModelName: "deepseek-chat", APIKeyEnc: "enc"})
	r := telegramRouter(h)

	// 未配置 → 404
	w := doJSON(r, http.MethodGet, "/telegram", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unconfigured get want 404, got %d %s", w.Code, w.Body.String())
	}
	// 保存
	w = doJSON(r, http.MethodPost, "/telegram", `{"bot_token":"123456:ABC-DEF","model_id":"m1"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("save want 200, got %d %s", w.Code, w.Body.String())
	}
	// 配置后 Get → 200 + token 前缀脱敏
	w = doJSON(r, http.MethodGet, "/telegram", "")
	if w.Code != http.StatusOK {
		t.Fatalf("configured get want 200, got %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if contains(body, "123456:ABC-DEF") {
		t.Fatalf("bot token leaked: %s", body)
	}
	_, data := parseEnvelope(t, w)
	var cfg dto.TelegramConfigDTO
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("resp parse: %v", err)
	}
	if cfg.BotTokenPrefix == "" {
		t.Fatalf("want masked prefix, got %+v", cfg)
	}
}

// SetModel 引用不存在模型 → 404。
func TestTelegramSetModelNotFound(t *testing.T) {
	h, st := newTelegramHandler(t)
	st.CreateModel(&model.AIModel{ID: "m1", UserID: "single-user", Provider: "deepseek", ModelName: "deepseek-chat", APIKeyEnc: "enc"})
	r := telegramRouter(h)
	doJSON(r, http.MethodPost, "/telegram", `{"bot_token":"123456:ABC","model_id":"m1"}`)

	w := doJSON(r, http.MethodPost, "/telegram/model", `{"model_id":"ghost"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("set model ghost want 404, got %d %s", w.Code, w.Body.String())
	}
}

// ========== crypto ==========

func newCryptoHandler(t *testing.T) (*CryptoHandler, *crypto.KeyManager) {
	t.Helper()
	km, err := crypto.NewKeyManager("")
	if err != nil {
		t.Fatal(err)
	}
	return NewCryptoHandler(km), km
}

// 解密：aad 不匹配 → 400 + 1001（防密文跨端点重放）。
func TestCryptoDecryptAADMismatch(t *testing.T) {
	h, _ := newCryptoHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/crypto/decrypt", h.Decrypt)

	ts := time.Now().UnixMilli()
	body := `{"ts":"` + itoa(ts) + `","aad":"wrong-aad","wrapped_key":"a","iv":"a","ciphertext":"a"}`
	w := doJSON(r, http.MethodPost, "/crypto/decrypt", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("aad mismatch want 400, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1001 {
		t.Fatalf("want 1001, got %d", code)
	}
}

// 解密：ts 过期（±5min 外）→ 400。
func TestCryptoDecryptStaleTS(t *testing.T) {
	h, _ := newCryptoHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/crypto/decrypt", h.Decrypt)

	old := time.Now().Add(-10 * time.Minute).UnixMilli()
	body := `{"ts":"` + itoa(old) + `","aad":"` + itoa(old) + `/crypto/decrypt","wrapped_key":"a","iv":"a","ciphertext":"a"}`
	w := doJSON(r, http.MethodPost, "/crypto/decrypt", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("stale ts want 400, got %d %s", w.Code, w.Body.String())
	}
}

// ========== backtest ==========

// newBacktestHandler 组装 backtest handler（engine 用 fake models/AI + mock klines）。
func newBacktestHandler(t *testing.T) (*BacktestHandler, *backtest.Engine) {
	t.Helper()
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	models := &debateFakeModels{models: map[string]*llm.Model{
		"m1": {Provider: "gpt", ModelName: "gpt-4o", APIKey: "sk-test"},
	}}
	ai := llm.NewWithTransport(&debateFakeAI{body: `{"choices":[{"message":{"content":"<decision>[{\"action\":\"hold\"}]</decision>"}}]}`}, 5*time.Second)
	eng := backtest.NewEngine(st, &mockKlinesForHandler{}, models, ai)
	return NewBacktestHandler(eng), eng
}

// 简化：backtest 需要 provider.KlineProvider 接口，用最小实现。
type mockKlinesForHandler struct{}

func (m *mockKlinesForHandler) Klines(ctx context.Context, symbol, interval string, limit int) ([]dto.KlineDTO, error) {
	return nil, nil
}
func (m *mockKlinesForHandler) Name() string { return "mock" }

// backtest 启动：缺 symbols → 400 + 1001。
func TestBacktestStartMissingSymbols(t *testing.T) {
	h, _ := newBacktestHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/backtest/start", h.Start)

	w := doJSON(r, http.MethodPost, "/backtest/start", `{"config":{"initial_balance":10000}}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing symbols want 400, got %d %s", w.Code, w.Body.String())
	}
	code, _ := parseEnvelope(t, w)
	if code != 1001 {
		t.Fatalf("want 1001, got %d", code)
	}
}

// 时间窗倒挂 → 400。
func TestBacktestStartTimeInverted(t *testing.T) {
	h, _ := newBacktestHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/backtest/start", h.Start)

	now := time.Now().Unix()
	body := `{"config":{"symbols":["BTC-USDT"],"start_time":` + itoa(now) + `,"end_time":` + itoa(now-3600) + `}}`
	w := doJSON(r, http.MethodPost, "/backtest/start", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("inverted window want 400, got %d %s", w.Code, w.Body.String())
	}
}

// 运行中回测锁：第二次 start → 1411（同用户）。
func TestBacktestStartLockConflict(t *testing.T) {
	h, eng := newBacktestHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/backtest/start", h.Start)
	r.POST("/backtest/:action", h.Control)

	body := `{"config":{"symbols":["BTC-USDT"],"replay_only":true,"start_time":` + itoa(time.Now().Add(-3600*time.Second).Unix()) + `,"end_time":` + itoa(time.Now().Unix()) + `}}`
	w1 := doJSON(r, http.MethodPost, "/backtest/start", body)
	if w1.Code != http.StatusOK {
		t.Fatalf("first start want 200, got %d %s", w1.Code, w1.Body.String())
	}
	w2 := doJSON(r, http.MethodPost, "/backtest/start", body)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("second start want 400 (1411), got %d %s", w2.Code, w2.Body.String())
	}
	code, _ := parseEnvelope(t, w2)
	if code != 1411 {
		t.Fatalf("want 1411, got %d", code)
	}
	// 收尾：停掉 run 释放锁
	var meta struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &meta)
	if meta.Data.ID != "" {
		doJSON(r, http.MethodPost, "/backtest/stop", `{"run_id":"`+meta.Data.ID+`"}`)
	}
	_ = eng
}

// ========== helpers ==========

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

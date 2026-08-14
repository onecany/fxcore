package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/llm"
	"fxcore/internal/model"
	"fxcore/internal/store"
)

// strategyFakeModels 实现 llm.ModelProvider（TestRun 解密 key 路径）。
type strategyFakeModels struct {
	models map[string]*llm.Model
}

func (f *strategyFakeModels) GetModel(id string) (*llm.Model, bool) {
	m, ok := f.models[id]
	return m, ok
}

// strategyFakeAI 可配置响应体的 RoundTripper（带 10ms 延迟，让 latency 断言非零）。
type strategyFakeAI struct {
	status int
	body   string
}

func (f *strategyFakeAI) RoundTrip(req *http.Request) (*http.Response, error) {
	time.Sleep(10 * time.Millisecond)
	return &http.Response{
		StatusCode: f.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}

// strategyTestEnv 组装完整 handler 依赖（内存 store + fake models + fake AI）。
// store 里同步建 m1 模型（归属校验走 store；llm.ModelProvider 只负责解密 key）。
func strategyTestEnv(t *testing.T, aiBody string) (*StrategyHandler, *store.Store) {
	t.Helper()
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// store 里的模型记录（msvc.Get 归属校验用）；UserID 与无 claims 的 currentUserID 回退 "single-user" 对齐
	st.CreateModel(&model.AIModel{
		ID:        "m1",
		UserID:    "single-user",
		Name:      "test-model",
		Provider:  "gpt",
		ModelName: "gpt-4o",
	})
	models := &strategyFakeModels{models: map[string]*llm.Model{
		"m1": {Provider: "gpt", ModelName: "gpt-4o", APIKey: "sk-test"},
	}}
	ai := llm.NewWithTransport(&strategyFakeAI{status: 200, body: aiBody}, 5*time.Second)
	msvc := service.NewModelService(st, nil)
	h := NewStrategyHandler(service.NewStrategyService(st), msvc, ai, models)
	return h, st
}

func strategyRouter(h *StrategyHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/strategies/test-run", h.TestRun)
	return r
}

// testRunReq 构造 TestRun 请求（config 为最小合法配置，model_id 可覆盖）。
func testRunReq(modelID string) string {
	return `{"config":{"strategy_type":"ai","language":"zh","coin_source":{"source_type":"static","static_coins":["BTC-USDT"]},"risk_control":{"max_positions":3}},"model_id":"` + modelID + `"}`
}

// 成功路径：AI 返回合法 XML 决策 → parsed=true + 决策数组 + prompt 非空。
func TestStrategyTestRunParsed(t *testing.T) {
	aiBody := `{"choices":[{"message":{"content":"<reasoning>bullish trend</reasoning><decision>[{\"action\":\"open_long\",\"symbol\":\"BTC-USDT\",\"quantity\":0.01,\"leverage\":5,\"confidence\":80}]</decision>"}}]}`
	h, _ := strategyTestEnv(t, aiBody)
	r := strategyRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/strategies/test-run", strings.NewReader(testRunReq("m1")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("test-run failed: %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data dto.TestRunResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("resp parse: %v", err)
	}
	if !env.Data.Parsed {
		t.Fatalf("want parsed=true, got false: %s", w.Body.String())
	}
	if len(env.Data.Decisions) != 1 {
		t.Fatalf("want 1 decision, got %d", len(env.Data.Decisions))
	}
	if env.Data.Decisions[0].Action != model.ActionOpenLong {
		t.Fatalf("want open_long, got %q", env.Data.Decisions[0].Action)
	}
	if env.Data.Prompt == "" || env.Data.Raw == "" {
		t.Fatalf("prompt/raw must be non-empty")
	}
	if env.Data.LatencyMS <= 0 {
		t.Fatalf("want latency>0, got %d", env.Data.LatencyMS)
	}
}

// 解析失败路径：AI 输出非决策文本 → 200 + parsed=false + raw 保留 + error 说明。
func TestStrategyTestRunParseFailed(t *testing.T) {
	aiBody := `{"choices":[{"message":{"content":"I think BTC will go up but I am not sure."}}]}`
	h, _ := strategyTestEnv(t, aiBody)
	r := strategyRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/strategies/test-run", strings.NewReader(testRunReq("m1")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("parse failure should still be 200: %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data dto.TestRunResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("resp parse: %v", err)
	}
	if env.Data.Parsed {
		t.Fatalf("want parsed=false for non-decision output")
	}
	if env.Data.Raw == "" {
		t.Fatalf("raw must be preserved on parse failure")
	}
	if env.Data.Error == "" {
		t.Fatalf("want error message on parse failure")
	}
}

// 模型不存在 → 1004。
func TestStrategyTestRunModelNotFound(t *testing.T) {
	h, _ := strategyTestEnv(t, `{"choices":[{"message":{"content":"x"}}]}`)
	r := strategyRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/strategies/test-run", strings.NewReader(testRunReq("ghost")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for unknown model, got %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Code int `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != 1004 {
		t.Fatalf("want code 1004, got %d", env.Code)
	}
}

// AI 服务失败 → 1301。
func TestStrategyTestRunAIError(t *testing.T) {
	h, _ := strategyTestEnv(t, `{"choices":[{"message":{"content":"x"}}]}`)
	h.ai = llm.NewWithTransport(&strategyFakeAI{status: 500, body: `{"error":{"message":"boom"}}`}, 5*time.Second)
	r := strategyRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/strategies/test-run", strings.NewReader(testRunReq("m1")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 for AI failure, got %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Code int `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != 1301 {
		t.Fatalf("want code 1301, got %d", env.Code)
	}
}

// 越权模型（他人模型）→ 404（归属校验；无 claims 时 currentUserID 回退 "single-user"，与 m-other 的属主不匹配）。
func TestStrategyTestRunModelOwnership(t *testing.T) {
	h, st := strategyTestEnv(t, `{"choices":[{"message":{"content":"x"}}]}`)
	// 造一个属于他人（user-other）的模型，验证 TestRun 以 single-user 访问返回 404
	st.CreateModel(&model.AIModel{
		ID:        "m-other",
		UserID:    "user-other",
		Name:      "other-model",
		Provider:  "gpt",
		ModelName: "gpt-4o",
	})
	r := strategyRouter(h)

	// 无 claims → currentUserID="single-user" ≠ "user-other" → 404
	req := httptest.NewRequest(http.MethodPost, "/strategies/test-run", strings.NewReader(testRunReq("m-other")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for foreign model, got %d %s", w.Code, w.Body.String())
	}
}

// 参数校验：缺 model_id → 1001。
func TestStrategyTestRunMissingModelID(t *testing.T) {
	h, _ := strategyTestEnv(t, `{"choices":[{"message":{"content":"x"}}]}`)
	r := strategyRouter(h)

	body := `{"config":{"strategy_type":"ai"}}`
	req := httptest.NewRequest(http.MethodPost, "/strategies/test-run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing model_id, got %d %s", w.Code, w.Body.String())
	}
}

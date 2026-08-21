package handler

import (
	"context"
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
// lastBody 记录最近一次请求体（断言 user 消息含市场数据用）。
type strategyFakeAI struct {
	status   int
	body     string
	lastBody string
}

func (f *strategyFakeAI) RoundTrip(req *http.Request) (*http.Response, error) {
	time.Sleep(10 * time.Millisecond)
	if b, err := io.ReadAll(req.Body); err == nil {
		f.lastBody = string(b)
	}
	return &http.Response{
		StatusCode: f.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}

// strategyTestEnv 组装完整 handler 依赖（内存 store + fake models + fake AI + fake klines）。
// store 里同步建 m1 模型（归属校验走 store；llm.ModelProvider 只负责解密 key）。
// 返回 handler、store、fakeAI（断言 user 消息内容用）。
func strategyTestEnv(t *testing.T, aiBody string) (*StrategyHandler, *store.Store, *strategyFakeAI) {
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
	fai := &strategyFakeAI{status: 200, body: aiBody}
	ai := llm.NewWithTransport(fai, 5*time.Second)
	msvc := service.NewModelService(st, nil)
	h := NewStrategyHandler(service.NewStrategyService(st), msvc, ai, models, &strategyFakeKlines{})
	return h, st, fai
}

// strategyFakeKlines 返回固定 K 线（20 根升序，便于断言 user 消息含市场数据）。
type strategyFakeKlines struct{}

func (f *strategyFakeKlines) Klines(ctx context.Context, symbol, interval string, limit int) ([]dto.KlineDTO, error) {
	out := make([]dto.KlineDTO, 0, 20)
	for i := 0; i < 20; i++ {
		out = append(out, dto.KlineDTO{
			Timestamp: int64(1700000000 + i*60),
			Open:      100, High: 101, Low: 99, Close: 100 + float64(i)*0.1, Volume: 1000,
		})
	}
	return out, nil
}

func (f *strategyFakeKlines) Name() string { return "fake" }

func strategyRouter(h *StrategyHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/strategies/test-run", h.TestRun)
	r.POST("/strategies/lint", h.Lint)
	return r
}

// testRunReq 构造 TestRun 请求（config 为最小合法配置，model_id 可覆盖）。
func testRunReq(modelID string) string {
	return `{"config":{"strategy_type":"ai","language":"zh","coin_source":{"source_type":"static","static_coins":["BTC-USDT"]},"risk_control":{"max_positions":3}},"model_id":"` + modelID + `"}`
}

// 成功路径：AI 返回合法 XML 决策 → parsed=true + 决策数组 + prompt 非空 + user 消息含市场数据。
func TestStrategyTestRunParsed(t *testing.T) {
	aiBody := `{"choices":[{"message":{"content":"<reasoning>bullish trend</reasoning><decision>[{\"action\":\"open_long\",\"symbol\":\"BTC-USDT\",\"quantity\":0.01,\"leverage\":5,\"confidence\":80}]</decision>"}}]}`
	h, _, fai := strategyTestEnv(t, aiBody)
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
	// user 消息必须含真实 K 线数据（2026-08 修复：此前是硬编码空话）——由 fakeAI.lastBody 断言，
	// 具体消息内容格式由 kernel.BuildKlineContext 单测覆盖
	if !contains(fai.lastBody, "Market data") {
		t.Fatalf("user message missing market data, got: %s", fai.lastBody)
	}
}

// 解析失败路径：AI 输出非决策文本 → 200 + parsed=false + raw 保留 + error 说明。
func TestStrategyTestRunParseFailed(t *testing.T) {
	aiBody := `{"choices":[{"message":{"content":"I think BTC will go up but I am not sure."}}]}`
	h, _, _ := strategyTestEnv(t, aiBody)
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
	// 契约锁：解析失败时 decisions 必须是 [] 而非 null（前端读 decisions.length 不崩）
	if env.Data.Decisions == nil {
		t.Fatalf("decisions must be empty array (not null) on parse failure, body: %s", w.Body.String())
	}
	if len(env.Data.Decisions) != 0 {
		t.Fatalf("want 0 decisions on parse failure, got %d", len(env.Data.Decisions))
	}
	if !contains(w.Body.String(), `"decisions":[]`) {
		t.Fatalf("JSON must serialize decisions as [], body: %s", w.Body.String())
	}
}

// 模型不存在 → 1004。
func TestStrategyTestRunModelNotFound(t *testing.T) {
	h, _, _ := strategyTestEnv(t, `{"choices":[{"message":{"content":"x"}}]}`)
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
	h, _, _ := strategyTestEnv(t, `{"choices":[{"message":{"content":"x"}}]}`)
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
	h, st, _ := strategyTestEnv(t, `{"choices":[{"message":{"content":"x"}}]}`)
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
	h, _, _ := strategyTestEnv(t, `{"choices":[{"message":{"content":"x"}}]}`)
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

// ========== Prompt Lint 契约测试 ==========

// lintResp 解析 lint 响应信封。
func lintResp(t *testing.T, body []byte) dto.LintResponse {
	t.Helper()
	var env struct {
		Data dto.LintResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("lint resp parse: %v (%s)", err, body)
	}
	return env.Data
}

// 合法配置（对齐默认值）→ 200 + issues 空数组（非 null）。
func TestStrategyLintClean(t *testing.T) {
	h, _, _ := strategyTestEnv(t, "")
	r := strategyRouter(h)

	body := `{"config":{"strategy_type":"ai","language":"zh","prompt_variant":"balanced","coin_source":{"source_type":"static","static_coins":["BTC-USDT"]},"indicators":{"klines":{"primary_timeframe":"15m","primary_count":200},"enable_raw_klines":true,"enable_ema":true,"ema_periods":[7,25,99]},"risk_control":{"max_positions":3,"btc_eth_max_leverage":5,"altcoin_max_leverage":5,"max_margin_usage":0.3,"min_risk_reward_ratio":1.5,"min_confidence":0.6},"prompt_sections":{"role_definition":"你是资深交易员。","trading_frequency":"每日最多 3 笔。","entry_standards":"EMA 金叉才进场。","decision_process":"先分析再决策。"}}}`
	req := httptest.NewRequest(http.MethodPost, "/strategies/lint", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", w.Code, w.Body.String())
	}
	data := lintResp(t, w.Body.Bytes())
	if data.Issues == nil || len(data.Issues) != 0 {
		t.Fatalf("clean config want empty issues array (non-null), got %+v", data.Issues)
	}
}

// 静态币源空 + custom_prompt 跳过分析 → 命中两类规则。
// 注意 risk_control 只传 max_positions（部分替换语义）也会因风险零值命中 lint——那正是 lint 的价值。
func TestStrategyLintFindsIssues(t *testing.T) {
	h, _, _ := strategyTestEnv(t, "")
	r := strategyRouter(h)

	body := `{"config":{"coin_source":{"source_type":"static","static_coins":[]},"risk_control":{"max_positions":3},"custom_prompt":"只输出决策，不要分析。"}}`
	req := httptest.NewRequest(http.MethodPost, "/strategies/lint", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", w.Code, w.Body.String())
	}
	data := lintResp(t, w.Body.Bytes())
	byCode := map[string]dto.LintIssue{}
	for _, it := range data.Issues {
		byCode[it.Code] = it
	}
	if _, ok := byCode[dto.CodeCoinStaticEmpty]; !ok {
		t.Fatalf("want coin_static_empty hit, got %+v", data.Issues)
	}
	if _, ok := byCode[dto.CodeTextContract]; !ok {
		t.Fatalf("want text_contract hit, got %+v", data.Issues)
	}
	// 部分替换 risk_control 后其余字段为零值，lint 应同样命中（这是「配置会被静默误解」的核心价值）
	if _, ok := byCode[dto.CodeRiskLeverageZero]; !ok {
		t.Fatalf("want risk_leverage_zero hit for partial risk_control, got %+v", data.Issues)
	}
	if _, ok := byCode[dto.CodeRiskMarginZero]; !ok {
		t.Fatalf("want risk_margin_zero hit for partial risk_control, got %+v", data.Issues)
	}
	for _, it := range data.Issues {
		if it.Severity != dto.SeverityError && it.Severity != dto.SeverityWarning {
			t.Fatalf("severity must be error|warning, got %q", it.Severity)
		}
		if it.Title == "" || it.Detail == "" || it.Field == "" {
			t.Fatalf("issue must carry title/detail/field, got %+v", it)
		}
	}
}

// 非法 body（非 JSON）→ 400。
func TestStrategyLintBadBody(t *testing.T) {
	h, _, _ := strategyTestEnv(t, "")
	r := strategyRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/strategies/lint", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for bad body, got %d %s", w.Code, w.Body.String())
	}
}

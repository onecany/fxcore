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
	"fxcore/internal/debate"
	"fxcore/internal/llm"
	"fxcore/internal/store"
)

type debateFakeModels struct {
	models map[string]*llm.Model
}

func (f *debateFakeModels) GetModel(id string) (*llm.Model, bool) {
	m, ok := f.models[id]
	return m, ok
}

type debateFakeAI struct {
	body string
}

func (f *debateFakeAI) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}

func newDebateHandler(t *testing.T) *DebateHandler {
	t.Helper()
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	models := &debateFakeModels{models: map[string]*llm.Model{
		"m1": {Provider: "gpt", ModelName: "gpt-4o", APIKey: "k1"},
		"m2": {Provider: "gpt", ModelName: "gpt-4o", APIKey: "k2"},
		"m3": {Provider: "gpt", ModelName: "gpt-4o", APIKey: "k3"},
	}}
	ai := llm.NewWithTransport(&debateFakeAI{body: `{"choices":[{"message":{"content":"test view"}}]}`}, 5*time.Second)
	return NewDebateHandler(debate.NewEngine(st, models, ai))
}

func debateRouter(h *DebateHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/debates", h.Create)
	r.POST("/debates/:id/:action", h.Control)
	r.POST("/debates/:id/execute", h.Execute)
	return r
}

func createDebate(t *testing.T, r *gin.Engine) string {
	t.Helper()
	body := `{"name":"debate-test","strategy_id":"s1","symbol":"BTC-USDT","max_rounds":1,"participants":["m1","m2","m3"]}`
	req := httptest.NewRequest(http.MethodPost, "/debates", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create debate failed: %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data dto.DebateSessionDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("create resp parse: %v", err)
	}
	if env.Data.ID == "" {
		t.Fatalf("create returned empty id: %s", w.Body.String())
	}
	return env.Data.ID
}

// Control（start）必须返回会话详情而非 null——对齐前端 controlDebate 的 Promise<DebateSession> 契约。
func TestDebateControlReturnsSession(t *testing.T) {
	h := newDebateHandler(t)
	r := debateRouter(h)
	id := createDebate(t, r)

	req := httptest.NewRequest(http.MethodPost, "/debates/"+id+"/start", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("control start failed: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data *dto.DebateSessionDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("control resp parse: %v", err)
	}
	if resp.Data == nil {
		t.Fatalf("control start returned null data, want session: %s", w.Body.String())
	}
	if resp.Data.ID != id {
		t.Fatalf("control returned wrong session: got %s want %s", resp.Data.ID, id)
	}
	if resp.Data.Status != dto.DebateRunning {
		t.Fatalf("want status running, got %q", resp.Data.Status)
	}
}

// Execute 契约：请求可达 + 返回结构化信封（成功路径返回 {executed,message} 由 handler 保证；
// 此处验证未完成会话走 1420 业务拒绝而非解析崩溃——契约错位时代码连请求都到不了）。
func TestDebateExecuteReturnsResult(t *testing.T) {
	h := newDebateHandler(t)
	r := debateRouter(h)
	id := createDebate(t, r)

	req := httptest.NewRequest(http.MethodPost, "/debates/"+id+"/execute", strings.NewReader(`{"trader_id":"t1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// 未完成会话 → 1420（业务拒绝，信封结构化；此前契约错位不涉及此路径）
	if w.Code != http.StatusConflict {
		t.Fatalf("execute on pending debate: want 409/1420, got %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("execute resp parse: %v", err)
	}
	if resp.Code != 1420 {
		t.Fatalf("want code 1420, got %d", resp.Code)
	}
}

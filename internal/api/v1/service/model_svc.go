// Package service 业务逻辑层（API设计.md 7.1 蓝图的 internal/api/v1/service/）。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"fxcore/httpclient"
	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/pkg/crypto"
	"fxcore/internal/store"
)

// ModelService AI 模型管理：RSA 加密存储、Key 轮换、连通测试。
type ModelService struct {
	store  *store.Store
	km     *crypto.KeyManager
	client *http.Client // SSRF 防护的探测客户端
}

// NewModelService 构造模型服务。
func NewModelService(s *store.Store, km *crypto.KeyManager) *ModelService {
	return &ModelService{
		store:  s,
		km:     km,
		client: newProbeClient(), // SSRF 防护：拨号层 IP 校验 + 禁重定向
	}
}

// ValidProviders 支持提供商列表。
var ValidProviders = map[string]bool{
	"deepseek": true, "qwen": true, "claude": true, "gpt": true, "gemini": true, "custom": true,
}

// Create 创建模型：API Key 用 RSA-OAEP 加密后落库（文档 6.2）。
func (svc *ModelService) Create(userID string, in *dto.CreateModelRequest) (*model.AIModel, *middleware.APIError) {
	if strings.TrimSpace(in.APIKey) == "" {
		return nil, middleware.BadRequest("api key required", map[string]string{"api_key": "must be provided on create"})
	}
	if !ValidProviders[in.Provider] {
		return nil, middleware.BadRequest("unsupported provider", map[string]string{"provider": "must be one of deepseek,qwen,claude,gpt,gemini,custom"})
	}
	enc, err := svc.km.Encrypt([]byte(in.APIKey))
	if err != nil {
		return nil, middleware.NewAPIError(middleware.CodeAIFailed, http.StatusInternalServerError, "encrypt api key failed: "+err.Error())
	}
	m := &model.AIModel{
		UserID: userID,
		Name:         in.Name,
		Provider:     in.Provider,
		ModelName:    in.ModelName,
		APIKeyEnc:    enc,
		APIKeyPrefix: maskKey(in.APIKey),
		Status:       "inactive",
		Config:       string(in.Config),
	}
	svc.store.CreateModel(m)
	// L1：返回深拷贝（GetModel 返回副本），避免 handler 锁外序列化与锁内改写竞争。
	created, ok := svc.store.GetModel(m.ID)
	if !ok {
		return nil, middleware.Internal("model created but not found")
	}
	return created, nil
}

// Update 更新模型。检测到新 Key 时用当前公钥重新加密并记录 rotated_at（文档 6.2 Key 轮换）。
func (svc *ModelService) Update(id, userID string, in *dto.CreateModelRequest) (*model.AIModel, *middleware.APIError) {
	m, ok := svc.store.GetModel(id)
	if !ok {
		return nil, middleware.NotFound("model not found")
	}
	if userID != "" && m.UserID != "" && m.UserID != userID {
		return nil, middleware.NotFound("model not found")
	}
	if !ValidProviders[in.Provider] {
		return nil, middleware.BadRequest("unsupported provider", map[string]string{"provider": "must be one of deepseek,qwen,claude,gpt,gemini,custom"})
	}
	m.Name = in.Name
	m.Provider = in.Provider
	m.ModelName = in.ModelName
	if len(in.Config) > 0 {
		m.Config = string(in.Config)
	}

	if in.APIKey != "" {
		plain, decErr := svc.km.Decrypt(m.APIKeyEnc)
		isNewKey := decErr != nil || string(plain) != in.APIKey
		if isNewKey {
			enc, err := svc.km.Encrypt([]byte(in.APIKey))
			if err != nil {
				return nil, middleware.NewAPIError(middleware.CodeAIFailed, http.StatusInternalServerError, "re-encrypt api key failed: "+err.Error())
			}
			m.APIKeyEnc = enc
			m.APIKeyPrefix = maskKey(in.APIKey)
			now := time.Now().UTC()
			m.RotatedAt = &now
		}
	}
	svc.store.UpdateModel(m)
	return m, nil
}

// Delete 软删除模型。
func (svc *ModelService) Delete(id, userID string) *middleware.APIError {
	m, ok := svc.store.GetModel(id)
	if !ok || (userID != "" && m.UserID != "" && m.UserID != userID) {
		return middleware.NotFound("model not found")
	}
	if !svc.store.DeleteModel(id) {
		return middleware.NotFound("model not found")
	}
	return nil
}

// List 全部未删除模型。
func (svc *ModelService) List(userID string) []*model.AIModel {
	return svc.store.ListModels(userID)
}

// Get 单个模型。
func (svc *ModelService) Get(id, userID string) (*model.AIModel, *middleware.APIError) {
	m, ok := svc.store.GetModel(id)
	if !ok {
		return nil, middleware.NotFound("model not found")
	}
	if userID != "" && m.UserID != "" && m.UserID != userID {
		return nil, middleware.NotFound("model not found")
	}
	return m, nil
}

// Test 连通测试：优先用请求内 keyOverride，否则解密已存 Key。
// 成功更新 last_test_at 与 status；超时归 1302，其余失败归 1301。
// L5：keyOverride 与已存 Key 不同时视为"临时 key 探测"，不写回模型状态/延迟
// （否则仪表盘健康度反映的是别的 key）。
func (svc *ModelService) Test(m *model.AIModel, keyOverride string) (int64, *middleware.APIError) {
	key := keyOverride
	isStoredKey := false
	if key == "" {
		plain, err := svc.km.Decrypt(m.APIKeyEnc)
		if err != nil {
			return 0, middleware.Internal("decrypt stored api key failed: " + err.Error())
		}
		key = string(plain)
		isStoredKey = true
	} else if plain, err := svc.km.Decrypt(m.APIKeyEnc); err == nil && string(plain) == keyOverride {
		isStoredKey = true // 与已存 Key 相同，测试结果可写回
	}

	start := time.Now()
	_, err := probeProvider(svc.client, m.Provider, key, m.Config)
	latency := time.Since(start).Milliseconds()

	// 仅当测试的是已存 Key 时才写回状态/延迟（L5）
	if isStoredKey {
		now := time.Now().UTC()
		m.LastTestAt = &now
		m.LastTestLatencyMS = latency
		if err != nil {
			m.Status = "error"
		} else {
			m.Status = "active"
		}
		svc.store.UpdateModel(m)
	}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return latency, middleware.AITimeout("AI provider timeout: " + err.Error())
		}
		return latency, middleware.AIFailed("AI provider failed: " + err.Error())
	}
	return latency, nil
}

// maskKey 脱敏：sk-****abcd。
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:3] + "****" + key[len(key)-4:]
}

// ========== 提供商探测（SSRF 防护） ==========

// newProbeClient 构造探测用 HTTP 客户端：
// 1. 禁重定向（S1：302 链可把请求导向内网，且 API Key 会随 Bearer 头发往攻击者 URL）
// 2. 拨号层 IP 校验（S1：拒绝私网/环回/链路本地/元数据网段，防 DNS rebinding）
// 3. 代理拨号放行（本地代理是信任的；SSRF 校验针对最终目标——CONNECT 代理
//    路径下 transport 先拨代理地址（如 127.0.0.1:7890），拦截会误伤本地代理）。
func newProbeClient() *http.Client {
	tr := httpclient.DefaultTransport().Clone()
	baseDial := tr.DialContext
	// 解析 CONNECT 代理地址（与 httpclient.resolveProxy 同序），代理拨号放行
	proxyHostPort := probeProxyHostPort()
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		// 代理地址放行（本地代理信任；SSRF 校验只针对最终请求目标）
		if proxyHostPort != "" && addr == proxyHostPort {
			return baseDial(ctx, network, addr)
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if isBlockedIP(ip) {
				return nil, fmt.Errorf("refuse dial to blocked IP %s (SSRF guard)", ip)
			}
		}
		return baseDial(ctx, network, addr)
	}
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 禁重定向
		},
	}
}

// probeProxyHostPort 返回 CONNECT 代理的 host:port（与 httpclient.resolveProxy
// 同序：ALL_PROXY > HTTPS_PROXY > HTTP_PROXY）；无代理返回 ""。
func probeProxyHostPort() string {
	for _, key := range []string{"ALL_PROXY", "HTTPS_PROXY", "HTTP_PROXY", "all_proxy", "https_proxy", "http_proxy"} {
		v := os.Getenv(key)
		if v == "" {
			continue
		}
		if u, err := url.Parse(v); err == nil && u.Host != "" {
			return u.Host
		}
	}
	return ""
}

// isBlockedIP 私网/环回/链路本地/组播/未指定地址一律拒绝。
// 169.254.169.254（云元数据）属于链路本地网段，由 IsLinkLocalUnicast 覆盖。
func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

// validateCustomBaseURL custom provider 的 base_url 预检（https-only + 解析后 IP 校验）。
// 拨号层还有第二道检查（防 DNS rebinding）。
func validateCustomBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}
	if u.Scheme != "https" {
		return errors.New("custom provider base_url must use https")
	}
	if u.Hostname() == "" {
		return errors.New("custom provider base_url missing host")
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return fmt.Errorf("resolve base_url host: %w", err)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("base_url resolves to blocked network (%s)", ip)
		}
	}
	return nil
}

// probeProvider 按提供商调用其 models 端点做连通测试。
func probeProvider(client *http.Client, provider, apiKey, cfgJSON string) (int, error) {
	switch provider {
	case "deepseek":
		return doGET(client, "https://api.deepseek.com/models", bearerHeader(apiKey))
	case "qwen":
		return doGET(client, "https://dashscope.aliyuncs.com/compatible-mode/v1/models", bearerHeader(apiKey))
	case "gpt":
		return doGET(client, "https://api.openai.com/v1/models", bearerHeader(apiKey))
	case "claude":
		return doGET(client, "https://api.anthropic.com/v1/models", map[string]string{
			"x-api-key":         apiKey,
			"anthropic-version": "2023-06-01",
		})
	case "gemini":
		return doGET(client, "https://generativelanguage.googleapis.com/v1beta/models?key="+url.QueryEscape(apiKey), nil)
	case "custom":
		var cfg struct {
			BaseURL string `json:"base_url"`
		}
		_ = json.Unmarshal([]byte(cfgJSON), &cfg)
		if cfg.BaseURL == "" {
			return 0, errors.New("custom provider requires config.base_url")
		}
		if err := validateCustomBaseURL(cfg.BaseURL); err != nil {
			return 0, err // SSRF 防护：拒绝内网/非 https
		}
		return doGET(client, strings.TrimRight(cfg.BaseURL, "/")+"/models", bearerHeader(apiKey))
	default:
		return 0, fmt.Errorf("unsupported provider %q", provider)
	}
}

func bearerHeader(key string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + key}
}

func doGET(client *http.Client, target string, headers map[string]string) (int, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", "fxcore/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 400 {
		return resp.StatusCode, fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func isTimeout(err error) bool {
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline")
}

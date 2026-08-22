// fxcore 后端并发压测工具
//   A. read 端点 200 并发瞬时突发 -> 验证限流(120/min)放行数精确 + 并发下无 5xx
//   B. 公开端点(openapi3.json)吞吐梯度(128/256/512 并发) -> 吞吐/延迟分位/饱和点
//   C. write 端点 30 并发签名写(POST /models) -> 验签+nonce+落库并发正确性
// 用法：起服务后（PORT=6080，API-only 即可）直接运行；自动注册临时账号，
//       base 常量按实际服务端口修改。签名契约 = ts+path+body+nonce，HMAC-SHA256。
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const base = "http://127.0.0.1:6080/api/v1"

var (
	signSecret string
	httpClient *http.Client
	errPrinted int64
)

type statusAgg struct {
	count int
	total time.Duration
}

func initClient() {
	jar, _ := cookiejar.New(nil)
	httpClient = &http.Client{
		Timeout:   30 * time.Second,
		Jar:       jar,
		Transport: &http.Transport{MaxIdleConnsPerHost: 256, MaxIdleConns: 512},
	}
}

func randNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// 签名串 = ts + path + body + nonce，HMAC-SHA256(sign_secret)
func sign(method, path string, body []byte, nonce string) (string, string) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(signSecret))
	_, _ = mac.Write([]byte(ts + path + string(body) + nonce))
	return ts, hex.EncodeToString(mac.Sum(nil))
}

func doReq(method, path string, body []byte, signed bool) (int, time.Duration, []byte) {
	start := time.Now()
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, _ := http.NewRequest(method, base+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if signed {
		nonce := randNonce()
		ts, sig := sign(method, "/api/v1"+path, body, nonce)
		req.Header.Set("X-Timestamp", ts)
		req.Header.Set("X-Nonce", nonce)
		req.Header.Set("X-Signature", sig)
	}
	resp, err := httpClient.Do(req)
	dur := time.Since(start)
	if err != nil {
		if atomic.AddInt64(&errPrinted, 1) <= 5 {
			fmt.Printf("    [client error] %s %s: %v\n", method, path, err)
		}
		return 0, dur, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, dur, b
}

func register(email string) bool {
	payload := map[string]any{"email": email, "password": "loadtest-pass-1", "nickname": "loadtest"}
	b, _ := json.Marshal(payload)
	code, _, resp := doReq("POST", "/auth/register", b, false)
	if code != 200 {
		fmt.Printf("register failed: %d %s\n", code, string(resp))
		return false
	}
	var env struct {
		Data struct {
			SignSecret string `json:"sign_secret"`
		} `json:"data"`
	}
	_ = json.Unmarshal(resp, &env)
	signSecret = env.Data.SignSecret
	return signSecret != ""
}

type stat struct {
	mu    sync.Mutex
	codes map[int]*statusAgg
	errs  int
	durs  []time.Duration
}

func newStat() *stat { return &stat{codes: map[int]*statusAgg{}} }

func (s *stat) add(code int, d time.Duration, clientErr bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if clientErr {
		s.errs++
		return
	}
	if a, ok := s.codes[code]; ok {
		a.count++
		a.total += d
	} else {
		s.codes[code] = &statusAgg{count: 1, total: d}
	}
	s.durs = append(s.durs, d)
}

func (s *stat) report(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("\n=== %s ===\n", name)
	for code, a := range s.codes {
		avg := time.Duration(int64(a.total) / int64(max(a.count, 1)))
		fmt.Printf("  HTTP %d: %d  (avg %v)\n", code, a.count, avg)
	}
	if s.errs > 0 {
		fmt.Printf("  client errors: %d\n", s.errs)
	}
	if len(s.durs) > 0 {
		sort.Slice(s.durs, func(i, j int) bool { return s.durs[i] < s.durs[j] })
		n := len(s.durs)
		p := func(q float64) time.Duration { return s.durs[int(float64(n-1)*q)] }
		fmt.Printf("  latency: p50 %v | p95 %v | p99 %v | max %v (n=%d)\n",
			p(0.50), p(0.95), p(0.99), s.durs[n-1], n)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Test A: read 端点 200 并发瞬时突发。限流 120/min -> 精确放行 120，其余 429，无 5xx
func testReadBurst() {
	const n = 200
	s := newStat()
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, dur, _ := doReq("GET", "/strategies", nil, false)
			s.add(code, dur, code == 0)
		}()
	}
	wg.Wait()
	fmt.Printf("  wall: %v\n", time.Since(start))
	s.report("A. read burst: 200 concurrent GET /strategies (limit 120/min)")
}

// Test B: 公开端点高并发吞吐
func testOpenSpec(workers, rounds int) {
	s := newStat()
	var wg sync.WaitGroup
	var inflight atomic.Int64
	start := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				inflight.Add(1)
				code, dur, _ := doReq("GET", "/openapi3.json", nil, false)
				inflight.Add(-1)
				s.add(code, dur, code == 0)
			}
		}()
	}
	wg.Wait()
	wall := time.Since(start)
	total := s.codes[200]
	s.report(fmt.Sprintf("B. openapi3.json: %d workers x %d rounds", workers, rounds))
	if total != nil {
		fmt.Printf("  throughput: %.0f req/s (wall %v, %d ok)\n",
			float64(total.count)/wall.Seconds(), wall, total.count)
	}
}

// Test C: write 端点 30 并发签名写模型
func testWriteBurst() {
	const n = 30
	s := newStat()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload := map[string]any{
				"name":       fmt.Sprintf("load-model-%d", i),
				"provider":   "deepseek",
				"model_name": "deepseek-chat",
				"api_key":    fmt.Sprintf("sk-loadtest-%d", i),
			}
			b, _ := json.Marshal(payload)
			code, dur, _ := doReq("POST", "/models", b, true)
			s.add(code, dur, code == 0)
		}(i)
	}
	wg.Wait()
	s.report("C. write burst: 30 concurrent signed POST /models (limit 30/min)")
}

func main() {
	initClient()
	email := fmt.Sprintf("load-%d@fxcore.local", time.Now().UnixNano())
	if !register(email) {
		os.Exit(1)
	}
	fmt.Printf("registered %s, sign_secret ok\n", email)

	// 预热连接
	doReq("GET", "/strategies", nil, false)
	time.Sleep(300 * time.Millisecond)

	testReadBurst()
	// 梯度对照：验证 reset 是否与并发连接数（somaxconn=128）相关
	for _, w := range []int{128, 256, 512} {
		testOpenSpec(w, 5)
	}
	testWriteBurst()

	fmt.Println("\nwrite verification: check DB via sqlite (see script output)")
}

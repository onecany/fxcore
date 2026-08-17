// Package cache 提供 Redis 与内存双实现的 Token 存储与滑动窗口限流器。
// 文档 6.1/6.3 要求 Redis；REDIS_URL 未配置或连接失败时自动降级为内存实现
// （单机可用，多实例部署必须切 Redis）。
package cache

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ========== TokenStore（Refresh Token 存储，文档 6.1） ==========

// TokenStore 接口。
type TokenStore interface {
	Set(ctx context.Context, token, userID string, ttl time.Duration) error
	Get(ctx context.Context, token string) (string, bool)
	Del(ctx context.Context, token string) error
	// Rotate 原子轮换：仅当旧 token 存在时删除旧并写入新（TTL 重置），
	// 返回旧 token 归属的 userID。并发用同一 token 刷新时只有一个成功（S4）。
	Rotate(ctx context.Context, oldToken, newToken string, ttl time.Duration) (userID string, ok bool, err error)
}

// NewTokenStore 构造 refresh token 存储：优先 Redis，失败降级内存。
// 命名空间 fx:refresh:——与密码重置令牌（fx:reset:）物理隔离，
// 防止重置令牌被 /auth/refresh 的 Rotate 当作 refresh token 消费（安全隔离）。
func NewTokenStore(redisURL string) TokenStore {
	return newKVTokenStore(redisURL, "fx:refresh:")
}

// ResetStore 密码重置令牌存储（一次性：Get 命中后成功重置即 Del）。
// 独立于 TokenStore 命名空间，杜绝跨用途消费（见 NewTokenStore 注释）。
type ResetStore interface {
	Set(ctx context.Context, token, userID string, ttl time.Duration) error
	Get(ctx context.Context, token string) (string, bool)
	Del(ctx context.Context, token string) error
}

// NewResetStore 构造密码重置令牌存储（前缀 fx:reset:）。
func NewResetStore(redisURL string) ResetStore {
	return newKVTokenStore(redisURL, "fx:reset:")
}

// ---- 通用命名空间实现（refresh / password reset 共用） ----

// kvTokenStore 命名空间化的 token 存储：Redis 或内存，键统一带前缀。
// 同时实现 TokenStore（含 Rotate）与 ResetStore（Set/Get/Del）。
type kvTokenStore struct {
	prefix string
	rdb    *redis.Client
	mu     sync.Mutex
	items  map[string]memToken
}

func newKVTokenStore(redisURL, prefix string) *kvTokenStore {
	if c := dialRedis(redisURL); c != nil {
		return &kvTokenStore{prefix: prefix, rdb: c}
	}
	log.Printf("[cache] token store: in-memory fallback (set REDIS_URL for multi-instance)")
	return &kvTokenStore{prefix: prefix, items: map[string]memToken{}}
}

func (s *kvTokenStore) Set(ctx context.Context, token, userID string, ttl time.Duration) error {
	if s.rdb != nil {
		return s.rdb.Set(ctx, s.prefix+token, userID, ttl).Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// L5：防无界增长——超阈值时清理过期条目（过期后通常不再被访问，惰性清理回收不了）
	if len(s.items) > maxMemKeys {
		now := time.Now()
		for k, t := range s.items {
			if now.After(t.exp) {
				delete(s.items, k)
			}
		}
	}
	s.items[token] = memToken{userID: userID, exp: time.Now().Add(ttl)}
	return nil
}

func (s *kvTokenStore) Get(ctx context.Context, token string) (string, bool) {
	if s.rdb != nil {
		v, err := s.rdb.Get(ctx, s.prefix+token).Result()
		if err != nil {
			return "", false
		}
		return v, true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[token]
	if !ok {
		return "", false
	}
	if time.Now().After(t.exp) {
		delete(s.items, token)
		return "", false
	}
	return t.userID, true
}

func (s *kvTokenStore) Del(ctx context.Context, token string) error {
	if s.rdb != nil {
		return s.rdb.Del(ctx, s.prefix+token).Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, token)
	return nil
}

// rotateScript 原子轮换：GET 校验 + DEL + SET 一步完成（Redis 单线程保证原子性），
// 返回旧 token 归属的 userID（不存在返回 nil）。
const rotateScript = `
local uid = redis.call('GET', KEYS[1])
if uid then
  redis.call('DEL', KEYS[1])
  redis.call('SET', KEYS[2], uid, 'EX', ARGV[1])
  return uid
end
return false
`

func (s *kvTokenStore) Rotate(ctx context.Context, oldToken, newToken string, ttl time.Duration) (string, bool, error) {
	if s.rdb != nil {
		r := s.rdb.Eval(ctx, rotateScript,
			[]string{s.prefix + oldToken, s.prefix + newToken},
			int(ttl.Seconds()))
		if r.Err() != nil {
			return "", false, r.Err()
		}
		v, err := r.Text()
		if err != nil {
			if err == redis.Nil {
				return "", false, nil
			}
			return "", false, err
		}
		return v, true, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[oldToken]
	if !ok || time.Now().After(t.exp) {
		return "", false, nil
	}
	delete(s.items, oldToken)
	s.items[newToken] = memToken{userID: t.userID, exp: time.Now().Add(ttl)}
	return t.userID, true, nil
}

// ---- 内存辅助类型 ----

type memToken struct {
	userID string
	exp    time.Time
}

// ========== RateLimiter（滑动窗口，文档 6.3） ==========

// RateLimiter 接口。Allow 返回 (是否放行, 重试等待秒数)。
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int)
}

// NewRateLimiter 构造：优先 Redis，失败降级内存。
func NewRateLimiter(redisURL string) RateLimiter {
	if c := dialRedis(redisURL); c != nil {
		return &redisLimiter{rdb: c}
	}
	log.Println("[cache] rate limiter: in-memory fallback (set REDIS_URL for multi-instance)")
	return &memLimiter{keys: map[string][]int64{}}
}

// ---- Redis 实现（ZSET 滑动窗口） ----

type redisLimiter struct {
	rdb *redis.Client
}

func (l *redisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int) {
	rk := "fx:rl:" + key
	now := time.Now().UnixMilli()
	windowMS := window.Milliseconds()
	start := now - windowMS

	pipe := l.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, rk, "0", fmt.Sprintf("%d", start))
	pipe.ZCard(ctx, rk)
	results, err := pipe.Exec(ctx)
	if err != nil {
		// Redis 异常时放行（fail-open），避免拖垮业务
		log.Printf("[cache] rate limit redis error: %v", err)
		return true, 0
	}
	count := results[1].(*redis.IntCmd).Val()
	if count >= int64(limit) {
		oldest, _ := l.rdb.ZRangeWithScores(ctx, rk, 0, 0).Result()
		retry := 1
		if len(oldest) > 0 {
			retry = int((oldest[0].Score + float64(windowMS) - float64(now)) / 1000) + 1
			if retry < 1 {
				retry = 1
			}
		}
		return false, retry
	}
	member := fmt.Sprintf("%d-%d", now, rand.Int63())
	if err := l.rdb.ZAdd(ctx, rk, redis.Z{Score: float64(now), Member: member}).Err(); err == nil {
		l.rdb.Expire(ctx, rk, window+time.Minute)
	}
	return true, 0
}

// ---- 内存实现 ----

type memLimiter struct {
	mu   sync.Mutex
	keys map[string][]int64 // key -> 请求时间戳（毫秒）
}

func (l *memLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, int) {
	now := time.Now().UnixMilli()
	windowMS := window.Milliseconds()
	start := now - windowMS

	l.mu.Lock()
	defer l.mu.Unlock()

	// 防御：key 数超阈值时全量清理过期条目（L8：防伪造 key 缓慢内存泄漏）
	if len(l.keys) > maxMemKeys {
		for k, ts := range l.keys {
			if len(ts) == 0 || ts[len(ts)-1] < start {
				delete(l.keys, k)
			}
		}
	}

	ts := l.keys[key]
	kept := ts[:0]
	for _, t := range ts {
		if t >= start {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		retry := int((kept[0]+windowMS-now)/1000) + 1
		if retry < 1 {
			retry = 1
		}
		l.keys[key] = kept
		return false, retry
	}
	l.keys[key] = append(kept, now)
	return true, 0
}

// ========== NonceCache（签名重放防护，S6） ==========

// NonceCache 一次性 nonce 缓存：Add 返回 true 表示首次见到（可放行），false 表示重复。
type NonceCache interface {
	Add(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// NewNonceCache 构造：优先 Redis，失败降级内存。
func NewNonceCache(redisURL string) NonceCache {
	if c := dialRedis(redisURL); c != nil {
		return &redisNonceCache{rdb: c}
	}
	return &memNonceCache{items: map[string]time.Time{}}
}

type redisNonceCache struct {
	rdb *redis.Client
}

func (c *redisNonceCache) Add(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	// SETNX：key 已存在返回 false（重放）
	ok, err := c.rdb.SetNX(ctx, "fx:nonce:"+key, 1, ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

type memNonceCache struct {
	mu    sync.Mutex
	items map[string]time.Time
}

func (c *memNonceCache) Add(_ context.Context, key string, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) > maxMemKeys {
		now := time.Now()
		for k, exp := range c.items {
			if now.After(exp) {
				delete(c.items, k)
			}
		}
	}
	if exp, ok := c.items[key]; ok && time.Now().Before(exp) {
		return false, nil
	}
	c.items[key] = time.Now().Add(ttl)
	return true, nil
}

// maxMemKeys 内存缓存防御性上限（防伪造 key 无限增长）。
const maxMemKeys = 10000

// ========== 工具 ==========

func dialRedis(redisURL string) *redis.Client {
	if redisURL == "" {
		return nil
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("[cache] invalid REDIS_URL: %v", err)
		return nil
	}
	c := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		log.Printf("[cache] redis unreachable: %v", err)
		_ = c.Close()
		return nil
	}
	return c
}

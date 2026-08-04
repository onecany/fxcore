package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// S4：并发用同一 refresh token 刷新，原子 Rotate 必须恰好 1 个成功。
func TestTokenRotateConcurrent(t *testing.T) {
	ctx := context.Background()
	ts := NewTokenStore("") // 内存实现
	if err := ts.Set(ctx, "old-token", "u1", time.Hour); err != nil {
		t.Fatal(err)
	}

	const n = 24
	oks := make([]bool, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, oks[i], _ = ts.Rotate(ctx, "old-token", fmt.Sprintf("new-token-%d", i), time.Hour)
		}(i)
	}
	wg.Wait()

	success := 0
	for _, ok := range oks {
		if ok {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("expected exactly 1 rotate success, got %d", success)
	}

	// 旧 token 已作废；恰好 1 个新 token 存在且归属 u1
	if _, ok := ts.Get(ctx, "old-token"); ok {
		t.Fatal("old token should be revoked after rotate")
	}
	found := 0
	for i := 0; i < n; i++ {
		if uid, ok := ts.Get(ctx, fmt.Sprintf("new-token-%d", i)); ok {
			found++
			if uid != "u1" {
				t.Fatalf("rotated token belongs to %s, want u1", uid)
			}
		}
	}
	if found != 1 {
		t.Fatalf("expected exactly 1 rotated token to exist, got %d", found)
	}
}

// 轮换后旧 token 重放必须失败（1101 语义）。
func TestTokenRotateReplayRejected(t *testing.T) {
	ctx := context.Background()
	ts := NewTokenStore("")
	_ = ts.Set(ctx, "old", "u1", time.Hour)
	if _, ok, err := ts.Rotate(ctx, "old", "new", time.Hour); err != nil || !ok {
		t.Fatalf("first rotate should succeed: ok=%v err=%v", ok, err)
	}
	if _, ok, err := ts.Rotate(ctx, "old", "new2", time.Hour); err != nil || ok {
		t.Fatalf("replay of revoked token should fail: ok=%v err=%v", ok, err)
	}
}

// S6：nonce 缓存一次性语义（同 nonce 重复 Add 必须失败）。
func TestNonceCacheSingleUse(t *testing.T) {
	ctx := context.Background()
	nc := NewNonceCache("")
	first, err := nc.Add(ctx, "sig:u1:abc", time.Minute)
	if err != nil || !first {
		t.Fatalf("first nonce add should succeed: first=%v err=%v", first, err)
	}
	dup, err := nc.Add(ctx, "sig:u1:abc", time.Minute)
	if err != nil || dup {
		t.Fatalf("duplicate nonce should be rejected: dup=%v err=%v", dup, err)
	}
}

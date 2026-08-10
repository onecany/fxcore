package store

import (
	"path/filepath"
	"testing"

	"fxcore/internal/model"
)

// 多用户隔离断言（DB 双路径）：List/Create/Get 按 user 过滤。
func TestUserIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "iso.db")
	db, err := OpenDB(path, "")
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{AdminEmail: "a@x.com", AdminPassword: "pw"}, db)
	if err != nil {
		t.Fatal(err)
	}
	// 两个用户各建策略 + 交易员
	for _, uid := range []string{"u1", "u2"} {
		s.CreateStrategy(&model.Strategy{ID: uid + "-s", UserID: uid, Name: "s-" + uid})
		s.CreateTrader(&model.Trader{ID: uid + "-t", UserID: uid, Name: "t-" + uid, Status: model.StatusIdle})
	}
	// 列表按 user 过滤
	if got := len(s.ListStrategies("u1")); got != 1 {
		t.Fatalf("ListStrategies(u1) = %d, want 1", got)
	}
	if got := len(s.ListTraders("u2")); got != 1 {
		t.Fatalf("ListTraders(u2) = %d, want 1", got)
	}
	if got := len(s.ListStrategies("")); got != 2 {
		t.Fatalf("ListStrategies(all) = %d, want 2", got)
	}
	// Get 无 user 过滤（store 层原始读，service 层做归属校验）；空 userID List 放行历史数据
	if _, ok := s.GetStrategy("u2-s"); !ok {
		t.Fatal("GetStrategy 原始读应存在")
	}
}

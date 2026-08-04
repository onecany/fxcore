package dto

import (
	"math"
	"testing"
)

// L2：分页参数钳制——page=MaxInt64 不得溢出 (page-1)*size（曾导致切片越界 panic）。
func TestListQueryNormalizedClamps(t *testing.T) {
	q := &ListQuery{Page: math.MaxInt, Size: 100}
	page, size := q.Normalized()
	if page != 100000 {
		t.Fatalf("page=%d, want clamped 100000", page)
	}
	if size != 100 {
		t.Fatalf("size=%d, want 100", size)
	}

	// 验证钳制后乘法安全
	start := (page - 1) * size
	if start < 0 {
		t.Fatalf("start=%d overflowed to negative", start)
	}

	q2 := &ListQuery{Page: 0, Size: 0}
	p2, s2 := q2.Normalized()
	if p2 != 1 || s2 != 20 {
		t.Fatalf("zero defaults: page=%d size=%d, want 1/20", p2, s2)
	}

	q3 := &ListQuery{Page: -5, Size: 9999}
	p3, s3 := q3.Normalized()
	if p3 != 1 || s3 != 100 {
		t.Fatalf("negative/huge: page=%d size=%d, want 1/100", p3, s3)
	}
}

// FilterFields 字段过滤：白名单裁剪 + 空 fields 原样返回。
func TestFilterFields(t *testing.T) {
	in := map[string]any{"id": "1", "name": "x", "status": "idle"}
	out := FilterFields(in, []string{"id", "name"})
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("output type %T, want map", out)
	}
	if len(m) != 2 {
		t.Fatalf("filtered len=%d, want 2", len(m))
	}
	if _, has := m["status"]; has {
		t.Fatal("status should be filtered out")
	}

	// 空 fields -> 原样
	out2 := FilterFields(in, nil)
	if len(out2.(map[string]any)) != 3 {
		t.Fatal("empty fields should return original")
	}
}

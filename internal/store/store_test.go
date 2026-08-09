package store

import (
	"errors"
	"testing"
	"time"

	"fxcore/internal/model"
)

// L4：WithTrader 回调返回 error 时，map 内对象必须保持原样（副本机制）。
func TestWithTraderErrorLeavesNoMutation(t *testing.T) {
	s, err := New(Config{AdminEmail: "a@b.c", AdminPassword: "x", AdminSignSecret: "s"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.CreateTrader(&model.Trader{ID: "t1", Name: "orig", Status: model.StatusIdle})

	sentinel := errors.New("boom")
	_, err = s.WithTrader("t1", func(t *model.Trader) error {
		t.Name = "mutated" // 回调先改
		t.Status = model.StatusRunning
		return sentinel // 再报错
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v, want sentinel", err)
	}

	got, _ := s.GetTrader("t1")
	if got.Name != "orig" || got.Status != model.StatusIdle {
		t.Fatalf("map mutated despite error: name=%s status=%s", got.Name, got.Status)
	}
}

// L1：Create 后返回的必须是独立副本，外部修改不影响存储。
func TestCreateReturnsIndependentCopy(t *testing.T) {
	s, err := New(Config{AdminEmail: "a@b.c", AdminPassword: "x", AdminSignSecret: "s"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tr := &model.Trader{ID: "t2", Name: "n", Status: model.StatusIdle}
	s.CreateTrader(tr)

	cp, _ := s.GetTrader("t2")
	cp.Name = "hacked"
	cp.Status = model.StatusStopped

	again, _ := s.GetTrader("t2")
	if again.Name != "n" || again.Status != model.StatusIdle {
		t.Fatalf("copy not independent: name=%s status=%s", again.Name, again.Status)
	}
}

// L1：并发 Create + WithTrader 同一 trader 不产生数据竞争（-race 下跑）。
func TestCreateAndTransitionConcurrent(t *testing.T) {
	s, err := New(Config{AdminEmail: "a@b.c", AdminPassword: "x", AdminSignSecret: "s"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.CreateTrader(&model.Trader{ID: "t3", Name: "n", Status: model.StatusIdle})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			_, _ = s.GetTrader("t3") // 读副本（原 L1 竞争点：Create 返回存活指针）
		}
	}()
	for i := 0; i < 50; i++ {
		_, _ = s.WithTrader("t3", func(t *model.Trader) error {
			t.Status = model.StatusRunning
			time.Sleep(time.Microsecond)
			t.Status = model.StatusIdle
			return nil
		})
	}
	<-done
}

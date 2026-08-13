package service

import (
	"net/http"
	"testing"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/pkg/crypto"
)

// Create 无 api_key 必须被拒（binding 层已去掉 required，service 层兜底）。
// Update 留空 key 保留原 key 的语义依赖该校验只在 Create 生效。
func TestModelCreateRequiresAPIKey(t *testing.T) {
	s := newTestStore(t)
	km, err := crypto.NewKeyManager("")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewModelService(s, km)

	// 无 key → 1001
	_, apiErr := svc.Create("u1", &dto.CreateModelRequest{
		Name: "m", Provider: "deepseek", ModelName: "deepseek-chat", APIKey: "  ",
	})
	if apiErr == nil || apiErr.HTTP != http.StatusBadRequest {
		t.Fatalf("want BadRequest for empty api_key, got %+v", apiErr)
	}
	if apiErr.Code != middleware.CodeBadRequest {
		t.Fatalf("want code %d, got %d", middleware.CodeBadRequest, apiErr.Code)
	}

	// 带 key → 成功
	created, apiErr2 := svc.Create("u1", &dto.CreateModelRequest{
		Name: "m", Provider: "deepseek", ModelName: "deepseek-chat", APIKey: "sk-test",
	})
	if apiErr2 != nil {
		t.Fatalf("create with api_key failed: %+v", apiErr2)
	}
	if created.APIKeyPrefix == "" {
		t.Fatal("want api_key_prefix masked after create")
	}
}

// Update 留空 key 不报错（保留原 key，config 参数更新生效）。
func TestModelUpdateKeepsKeyWhenEmpty(t *testing.T) {
	s := newTestStore(t)
	km, err := crypto.NewKeyManager("")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewModelService(s, km)

	created, apiErr := svc.Create("u1", &dto.CreateModelRequest{
		Name: "m", Provider: "deepseek", ModelName: "deepseek-chat", APIKey: "sk-orig",
	})
	if apiErr != nil {
		t.Fatalf("create failed: %+v", apiErr)
	}

	// 留空 key + 改 config → 成功且不换 key
	updated, apiErr2 := svc.Update(created.ID, "u1", &dto.CreateModelRequest{
		Name: "m", Provider: "deepseek", ModelName: "deepseek-chat", APIKey: "",
		Config: []byte(`{"temperature":0.8}`),
	})
	if apiErr2 != nil {
		t.Fatalf("update with empty api_key failed: %+v", apiErr2)
	}
	if updated.APIKeyEnc != created.APIKeyEnc {
		t.Fatal("api_key should be preserved when update leaves it empty")
	}
	if updated.Config != `{"temperature":0.8}` {
		t.Fatalf("config not updated, got %q", updated.Config)
	}
}

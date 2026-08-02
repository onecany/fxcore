package mcp

import (
	"sync"
	"testing"
)

func TestRegisterProviderAndCreate(t *testing.T) {
	RegisterProvider("testprov", func(opts ...ClientOption) AIClient {
		return NewClient(append(opts, WithProvider("testprov"))...)
	})

	c := NewAIClientByProvider("testprov", WithMaxTokens(42))
	if c == nil {
		t.Fatal("NewAIClientByProvider returned nil for registered provider")
	}
	cl, ok := c.(*Client)
	if !ok {
		t.Fatalf("got %T, want *Client", c)
	}
	if cl.Provider != "testprov" {
		t.Errorf("Provider = %q, want testprov", cl.Provider)
	}
	if cl.MaxTokens != 42 {
		t.Errorf("MaxTokens = %d, want 42 (options forwarded)", cl.MaxTokens)
	}
}

func TestUnknownProviderReturnsNil(t *testing.T) {
	if c := NewAIClientByProvider("definitely-not-registered"); c != nil {
		t.Errorf("expected nil, got %T", c)
	}
}

func TestRegisterProviderIgnoresEmptyName(t *testing.T) {
	RegisterProvider("", func(opts ...ClientOption) AIClient { return NewClient() })
	if c := NewAIClientByProvider(""); c != nil {
		t.Errorf("expected nil for empty name, got %T", c)
	}
}

func TestProviderFactoryReceivesOptions(t *testing.T) {
	var got []ClientOption
	RegisterProvider("optcheck", func(opts ...ClientOption) AIClient {
		got = opts
		return NewClient(opts...)
	})
	o := WithMaxTokens(9)
	NewAIClientByProvider("optcheck", o)
	if len(got) != 1 {
		t.Fatalf("factory received %d options, want 1", len(got))
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	const workers = 16
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "conc-prov"
			RegisterProvider(name, func(opts ...ClientOption) AIClient {
				return NewClient(opts...)
			})
			for j := 0; j < 50; j++ {
				if c := NewAIClientByProvider(name); c == nil {
					t.Errorf("worker %d: provider not found", n)
					return
				}
				NewAIClientByProvider("not-registered") // must return nil safely
			}
		}(i)
	}
	wg.Wait()
}

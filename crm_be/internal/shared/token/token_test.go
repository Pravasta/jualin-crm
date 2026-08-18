package token_test

import (
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/token"
)

func TestGenerate_HashMatchesRaw(t *testing.T) {
	raw, hash, err := token.Generate()
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if raw == "" || hash == "" {
		t.Fatal("expected non-empty raw and hash")
	}
	if token.Hash(raw) != hash {
		t.Error("expected Hash(raw) to reproduce the same hash Generate returned")
	}
}

func TestGenerate_ProducesUniqueTokens(t *testing.T) {
	_, h1, _ := token.Generate()
	_, h2, _ := token.Generate()
	if h1 == h2 {
		t.Error("expected two generated tokens to differ")
	}
}

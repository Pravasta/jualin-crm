package password_test

import (
	"strings"
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/password"
)

func TestHashAndVerify_RoundTrip(t *testing.T) {
	hash, err := password.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("expected PHC-formatted hash, got %q", hash)
	}

	ok, err := password.Verify("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !ok {
		t.Error("expected correct password to verify")
	}
}

func TestVerify_WrongPassword(t *testing.T) {
	hash, err := password.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}

	ok, err := password.Verify("wrong password", hash)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if ok {
		t.Error("expected wrong password to fail verification")
	}
}

func TestHash_ProducesDifferentSaltEachTime(t *testing.T) {
	h1, _ := password.Hash("same password")
	h2, _ := password.Hash("same password")
	if h1 == h2 {
		t.Error("expected two hashes of the same password to differ (random salt)")
	}
}

func TestVerify_MalformedHash(t *testing.T) {
	_, err := password.Verify("anything", "not-a-valid-hash")
	if err != password.ErrInvalidHash {
		t.Errorf("expected ErrInvalidHash, got %v", err)
	}
}

package crypter_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/crypter"
)

// testKey is 32 bytes exactly — the minimum New accepts.
const testKey = "test-encryption-key-32-bytes-ok!"

func newTestCrypter(t *testing.T) *crypter.Crypter {
	t.Helper()
	c, err := crypter.New(testKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNew_RejectsShortKey(t *testing.T) {
	if _, err := crypter.New(strings.Repeat("a", crypter.MinKeyLength-1)); err == nil {
		t.Fatalf("expected a key one byte under MinKeyLength to be rejected")
	}
	if _, err := crypter.New(strings.Repeat("a", crypter.MinKeyLength)); err != nil {
		t.Fatalf("expected a key of exactly MinKeyLength to be accepted, got %v", err)
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c := newTestCrypter(t)

	// The real payload shape: a webhook signing secret.
	secret := []byte("whsec_kQ7vX2mNp8LzR4tYw1sA6dF3gH5jK9bC0eU7iO2pQ4M")

	sealed, err := c.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := c.Decrypt(sealed)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatalf("round trip changed the plaintext: got %q want %q", got, secret)
	}
}

// TestEncrypt_CiphertextNeverContainsPlaintext is the property migration
// 0009 actually depends on being true — the column holds something that
// a database dump alone cannot turn back into a usable signing secret.
func TestEncrypt_CiphertextNeverContainsPlaintext(t *testing.T) {
	c := newTestCrypter(t)
	secret := []byte("whsec_kQ7vX2mNp8LzR4tYw1sA6dF3gH5jK9bC0eU7iO2pQ4M")

	sealed, err := c.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Contains(sealed, secret) {
		t.Fatal("ciphertext contains the plaintext secret verbatim")
	}
}

// TestEncrypt_SamePlaintextTwiceDiffers proves the nonce is fresh per
// call. If it were fixed or derived, two identical secrets would produce
// identical columns and GCM's confidentiality would be gone.
func TestEncrypt_SamePlaintextTwiceDiffers(t *testing.T) {
	c := newTestCrypter(t)
	plaintext := []byte("whsec_same-input-both-times")

	first, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt first: %v", err)
	}
	second, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt second: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("encrypting the same plaintext twice produced identical output — the nonce is not fresh per call")
	}

	// Both must still decrypt back to the same plaintext.
	for i, sealed := range [][]byte{first, second} {
		got, err := c.Decrypt(sealed)
		if err != nil {
			t.Fatalf("Decrypt %d: %v", i, err)
		}
		if !bytes.Equal(got, plaintext) {
			t.Fatalf("Decrypt %d: got %q want %q", i, got, plaintext)
		}
	}
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	c := newTestCrypter(t)
	sealed, err := c.Encrypt([]byte("whsec_secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	other, err := crypter.New("a-completely-different-key-32byte")
	if err != nil {
		t.Fatalf("New other: %v", err)
	}
	if _, err := other.Decrypt(sealed); !errors.Is(err, crypter.ErrDecrypt) {
		t.Fatalf("expected ErrDecrypt with the wrong key, got %v", err)
	}
}

// TestDecrypt_TamperedInputFails walks every byte position and flips one
// bit in each. GCM must reject all of them — that is the property that
// makes it safe to store the result somewhere an operator can edit.
func TestDecrypt_TamperedInputFails(t *testing.T) {
	c := newTestCrypter(t)
	sealed, err := c.Encrypt([]byte("whsec_secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	for i := range sealed {
		tampered := bytes.Clone(sealed)
		tampered[i] ^= 0x01
		if _, err := c.Decrypt(tampered); !errors.Is(err, crypter.ErrDecrypt) {
			t.Fatalf("byte %d flipped: expected ErrDecrypt, got %v", i, err)
		}
	}
}

func TestDecrypt_TruncatedInputFails(t *testing.T) {
	c := newTestCrypter(t)
	sealed, err := c.Encrypt([]byte("whsec_secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	for _, tc := range []struct {
		name  string
		input []byte
	}{
		{"empty", nil},
		{"shorter than the nonce", sealed[:4]},
		{"nonce only, no ciphertext", sealed[:12]},
		{"last byte cut off", sealed[:len(sealed)-1]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c.Decrypt(tc.input); !errors.Is(err, crypter.ErrDecrypt) {
				t.Fatalf("expected ErrDecrypt, got %v", err)
			}
		})
	}
}

// TestEncrypt_EmptyPlaintext guards the boundary the webhook path will
// never hit but a future caller might.
func TestEncrypt_EmptyPlaintext(t *testing.T) {
	c := newTestCrypter(t)
	sealed, err := c.Encrypt(nil)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := c.Decrypt(sealed)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty plaintext back, got %q", got)
	}
}

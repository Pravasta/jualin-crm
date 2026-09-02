// Package crypter provides authenticated symmetric encryption for
// secrets this product must be able to READ BACK, as opposed to the
// three credentials it only ever verifies.
//
// api_key, public_key, and device_token are all held by someone else and
// presented to us; we hash them (Rule #20) and compare. A webhook signing
// secret is the opposite: WE hold it and use it as the HMAC key that
// proves an outgoing request came from us (Phase 7 TD §2). A one-way hash
// cannot serve as that key, which is what migration 0009 corrects.
//
// This is the only place in the codebase that encrypts rather than
// hashes, and it should stay that way — reach for hashing first, and use
// this only when the plaintext genuinely has to come back.
package crypter

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// MinKeyLength mirrors config's minJWTSecretLength /
// minFormTokenSecretLength — the key is a configured secret with the same
// entropy expectations, and one shared floor is easier to hold to than
// three different ones.
const MinKeyLength = 32

// ErrDecrypt is returned for every decryption failure without saying
// which one — wrong key, truncated input, and a tampered tag are
// deliberately indistinguishable to the caller. Distinguishing them tells
// an attacker which half of a forgery attempt was wrong.
var ErrDecrypt = errors.New("crypter: decrypt failed")

// Crypter encrypts with AES-256-GCM. GCM is authenticated: Decrypt fails
// on any modified byte rather than returning altered plaintext, which is
// what makes it safe to store the result in a column an operator can edit.
type Crypter struct {
	aead cipher.AEAD
}

// New derives the AES-256 key from key by SHA-256. The derivation exists
// so operators can supply an ordinary high-entropy string the way
// JWT_SECRET and FORM_TOKEN_SECRET already are, instead of exactly 32 raw
// bytes in some encoding. SHA-256 (not argon2id) is right here for the
// same reason it is for API keys: the input is a generated secret with
// full entropy, not a human-chosen password, so there is no offline
// guessing attack for a slow KDF to blunt.
func New(key string) (*Crypter, error) {
	if len(key) < MinKeyLength {
		return nil, fmt.Errorf("crypter: key must be at least %d bytes, got %d", MinKeyLength, len(key))
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("crypter: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypter: new gcm: %w", err)
	}
	return &Crypter{aead: aead}, nil
}

// Encrypt returns nonce‖ciphertext‖tag as one slice. The nonce is fresh
// per call from crypto/rand and stored alongside the ciphertext: GCM
// loses all confidentiality if a nonce is ever reused under the same key,
// so it is never derived from anything about the plaintext or the row.
func (c *Crypter) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypter: read nonce: %w", err)
	}
	// Seal appends to its first argument, so passing nonce puts the
	// nonce in front of the ciphertext in a single allocation.
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt reverses Encrypt. Every failure is ErrDecrypt — see its doc.
func (c *Crypter) Decrypt(sealed []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(sealed) < ns {
		return nil, ErrDecrypt
	}
	plaintext, err := c.aead.Open(nil, sealed[:ns], sealed[ns:], nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}

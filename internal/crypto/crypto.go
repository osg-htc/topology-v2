// Package crypto implements the envelope-encryption hierarchy, modeled on
// FabAID. A single 32-byte instance master key is the sole root of trust (no
// external KMS). HKDF-SHA256 derives domain-separated subordinate keys via
// distinct `info` labels:
//
//	topology-session-secret  -> OIDC-state / cookie HMAC secret
//	topology-pii-kek         -> KEK that wraps per-row PII (email) DEKs
//	topology-backup-key      -> base key for per-backup keys
//
// PII is protected with a two-level scheme: a fresh random Data Encryption Key
// (DEK) encrypts each value with AES-256-GCM, and the DEK itself is wrapped
// (encrypted) under the KEK. Compromising the database alone never reveals
// plaintext without the master key.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// HKDF info labels (domain separation).
const (
	labelSessionSecret = "topology-session-secret"
	labelPIIKEK        = "topology-pii-kek"
	labelBackupKey     = "topology-backup-key"
)

// Encryptor holds derived keys for PII envelope encryption.
type Encryptor struct {
	masterKey []byte
	piiKEK    []byte
}

// NewEncryptor derives the PII KEK from the master key.
func NewEncryptor(masterKey []byte) (*Encryptor, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	kek, err := deriveKey(masterKey, labelPIIKEK)
	if err != nil {
		return nil, err
	}
	return &Encryptor{masterKey: masterKey, piiKEK: kek}, nil
}

// deriveKey derives a 32-byte key from the master key using HKDF-SHA256 with the
// given info label.
func deriveKey(masterKey []byte, label string) ([]byte, error) {
	r := hkdf.New(sha256.New, masterKey, nil, []byte(label))
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("deriving key %q: %w", label, err)
	}
	return key, nil
}

// SessionSecret returns the HMAC secret used to sign OIDC-state cookies.
func SessionSecret(masterKey []byte) ([]byte, error) {
	return deriveKey(masterKey, labelSessionSecret)
}

// BackupKey derives a per-backup key from the backup base key, using the backup
// name as HKDF salt.
func BackupKey(masterKey []byte, backupName string) ([]byte, error) {
	base, err := deriveKey(masterKey, labelBackupKey)
	if err != nil {
		return nil, err
	}
	r := hkdf.New(sha256.New, base, []byte(backupName), []byte("topology-per-backup-key"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, err
	}
	return key, nil
}

// GenerateDEK returns a fresh 32-byte data encryption key.
func GenerateDEK() ([]byte, error) {
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return nil, err
	}
	return dek, nil
}

// gcmSeal encrypts plaintext with key using AES-256-GCM, prepending the nonce.
func gcmSeal(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plaintext, nil), nil
}

// gcmOpen reverses gcmSeal (nonce is prepended to ciphertext).
func gcmOpen(key, blob []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(blob) < g.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := blob[:g.NonceSize()], blob[g.NonceSize():]
	return g.Open(nil, nonce, ct, nil)
}

// Encrypt encrypts plaintext with a DEK (nonce prepended to the returned bytes).
func (e *Encryptor) Encrypt(dek, plaintext []byte) ([]byte, error) {
	return gcmSeal(dek, plaintext)
}

// Decrypt reverses Encrypt.
func (e *Encryptor) Decrypt(dek, ciphertext []byte) ([]byte, error) {
	return gcmOpen(dek, ciphertext)
}

// WrapDEK encrypts a DEK under the PII KEK.
func (e *Encryptor) WrapDEK(dek []byte) ([]byte, error) {
	return gcmSeal(e.piiKEK, dek)
}

// UnwrapDEK reverses WrapDEK.
func (e *Encryptor) UnwrapDEK(wrapped []byte) ([]byte, error) {
	return gcmOpen(e.piiKEK, wrapped)
}

// EncryptedField bundles the ciphertext and wrapped DEK for one PII value,
// suitable for persisting into three BYTEA columns.
type EncryptedField struct {
	Ciphertext []byte // nonce-prefixed AES-256-GCM ciphertext
	WrappedDEK []byte // DEK encrypted under the PII KEK (nonce-prefixed)
}

// EncryptPII generates a fresh DEK, encrypts the plaintext, and wraps the DEK.
func (e *Encryptor) EncryptPII(plaintext string) (*EncryptedField, error) {
	dek, err := GenerateDEK()
	if err != nil {
		return nil, err
	}
	ct, err := e.Encrypt(dek, []byte(plaintext))
	if err != nil {
		return nil, err
	}
	wrapped, err := e.WrapDEK(dek)
	if err != nil {
		return nil, err
	}
	return &EncryptedField{Ciphertext: ct, WrappedDEK: wrapped}, nil
}

// DecryptPII unwraps the DEK and decrypts the ciphertext.
func (e *Encryptor) DecryptPII(f *EncryptedField) (string, error) {
	if f == nil || len(f.Ciphertext) == 0 {
		return "", nil
	}
	dek, err := e.UnwrapDEK(f.WrappedDEK)
	if err != nil {
		return "", err
	}
	pt, err := e.Decrypt(dek, f.Ciphertext)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// EncryptConfigValue produces a self-describing "enc:v1:<base64>" string for
// storing secrets (e.g. the OIDC client secret) in app_config.
func (e *Encryptor) EncryptConfigValue(plaintext string) (string, error) {
	ct, err := gcmSeal(e.piiKEK, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return "enc:v1:" + base64.StdEncoding.EncodeToString(ct), nil
}

// DecryptConfigValue reverses EncryptConfigValue and passes through legacy
// plaintext values that lack the "enc:v1:" prefix.
func (e *Encryptor) DecryptConfigValue(value string) (string, error) {
	const prefix = "enc:v1:"
	if len(value) < len(prefix) || value[:len(prefix)] != prefix {
		return value, nil // plaintext passthrough
	}
	raw, err := base64.StdEncoding.DecodeString(value[len(prefix):])
	if err != nil {
		return "", err
	}
	pt, err := gcmOpen(e.piiKEK, raw)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// SignHMAC returns the base64url HMAC-SHA256 of msg under key.
func SignHMAC(key []byte, msg string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// VerifyHMAC checks a signature produced by SignHMAC in constant time.
func VerifyHMAC(key []byte, msg, sig string) bool {
	expected := SignHMAC(key, msg)
	return hmac.Equal([]byte(expected), []byte(sig))
}

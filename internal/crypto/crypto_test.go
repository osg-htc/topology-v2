package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func testMasterKey(t *testing.T, seed byte) []byte {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = seed + byte(i)
	}
	return k
}

func TestNewEncryptor_RejectsWrongKeySize(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		if _, err := NewEncryptor(make([]byte, n)); err == nil {
			t.Errorf("NewEncryptor with a %d-byte key: got no error, want one", n)
		}
	}
	if _, err := NewEncryptor(make([]byte, 32)); err != nil {
		t.Errorf("NewEncryptor with a 32-byte key: got %v, want no error", err)
	}
}

func TestDeriveKey_DeterministicAndDomainSeparated(t *testing.T) {
	mk := testMasterKey(t, 1)

	k1, err := deriveKey(mk, "label-a")
	if err != nil {
		t.Fatal(err)
	}
	k2, err := deriveKey(mk, "label-a")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Error("deriveKey is not deterministic for the same master key and label")
	}

	k3, err := deriveKey(mk, "label-b")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(k1, k3) {
		t.Error("deriveKey produced the same key for two different labels -- domain separation is broken")
	}

	otherMK := testMasterKey(t, 2)
	k4, err := deriveKey(otherMK, "label-a")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(k1, k4) {
		t.Error("deriveKey produced the same key for two different master keys")
	}
}

func TestGenerateDEK(t *testing.T) {
	d1, err := GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}
	if len(d1) != 32 {
		t.Fatalf("got %d bytes, want 32", len(d1))
	}
	d2, err := GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(d1, d2) {
		t.Error("two calls to GenerateDEK returned the same bytes -- randomness is broken")
	}
}

func TestEncryptor_EncryptDecrypt_RoundTrip(t *testing.T) {
	e, err := NewEncryptor(testMasterKey(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("some plaintext")

	ct, err := e.Encrypt(dek, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.Decrypt(dek, ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("round trip: got %q, want %q", got, plaintext)
	}

	wrongDEK, err := GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Decrypt(wrongDEK, ct); err == nil {
		t.Error("decrypting with the wrong DEK succeeded")
	}

	tampered := append([]byte{}, ct...)
	tampered[len(tampered)-1] ^= 0xFF
	if _, err := e.Decrypt(dek, tampered); err == nil {
		t.Error("decrypting tampered ciphertext succeeded")
	}
}

func TestWrapUnwrapDEK_RoundTrip(t *testing.T) {
	e, err := NewEncryptor(testMasterKey(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}

	wrapped, err := e.WrapDEK(dek)
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.UnwrapDEK(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, dek) {
		t.Errorf("round trip: got %x, want %x", got, dek)
	}

	other, err := NewEncryptor(testMasterKey(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.UnwrapDEK(wrapped); err == nil {
		t.Error("unwrapping under a different master key's KEK succeeded -- compromising one instance's DB would expose PII wrapped by any master key")
	}
}

func TestEncryptPII_DecryptPII_RoundTrip(t *testing.T) {
	e, err := NewEncryptor(testMasterKey(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	field, err := e.EncryptPII("someone@example.org")
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.DecryptPII(field)
	if err != nil {
		t.Fatal(err)
	}
	if got != "someone@example.org" {
		t.Errorf("round trip: got %q", got)
	}

	// Two encryptions of the same plaintext must use fresh, independent DEKs
	// (never reuse a DEK across rows), so ciphertext and wrapped-DEK must
	// both differ even for identical input.
	field2, err := e.EncryptPII("someone@example.org")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(field.Ciphertext, field2.Ciphertext) || bytes.Equal(field.WrappedDEK, field2.WrappedDEK) {
		t.Error("encrypting the same plaintext twice produced identical ciphertext or wrapped DEK -- DEKs are being reused")
	}

	if got, err := e.DecryptPII(nil); err != nil || got != "" {
		t.Errorf("DecryptPII(nil) = (%q, %v), want (\"\", nil)", got, err)
	}
	if got, err := e.DecryptPII(&EncryptedField{}); err != nil || got != "" {
		t.Errorf("DecryptPII(empty) = (%q, %v), want (\"\", nil)", got, err)
	}
}

func TestEncryptConfigValue_DecryptConfigValue_RoundTrip(t *testing.T) {
	e, err := NewEncryptor(testMasterKey(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := e.EncryptConfigValue("super-secret-client-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "enc:v1:") {
		t.Fatalf("encrypted value %q doesn't have the enc:v1: prefix", enc)
	}
	got, err := e.DecryptConfigValue(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got != "super-secret-client-secret" {
		t.Errorf("round trip: got %q", got)
	}

	// Legacy plaintext values (written before encryption was added) must
	// pass through unchanged, not be rejected or misinterpreted.
	if got, err := e.DecryptConfigValue("plain-legacy-value"); err != nil || got != "plain-legacy-value" {
		t.Errorf("legacy plaintext passthrough: got (%q, %v), want (\"plain-legacy-value\", nil)", got, err)
	}
	if got, err := e.DecryptConfigValue(""); err != nil || got != "" {
		t.Errorf("empty value passthrough: got (%q, %v)", got, err)
	}

	if _, err := e.DecryptConfigValue("enc:v1:not-valid-base64!!!"); err == nil {
		t.Error("decrypting malformed base64 after the prefix succeeded")
	}
}

func TestSignHMAC_VerifyHMAC(t *testing.T) {
	key := []byte("a signing key")
	sig := SignHMAC(key, "message")

	if !VerifyHMAC(key, "message", sig) {
		t.Error("VerifyHMAC rejected a signature it just produced")
	}
	if VerifyHMAC(key, "a different message", sig) {
		t.Error("VerifyHMAC accepted a signature for a different message")
	}
	if VerifyHMAC([]byte("a different key"), "message", sig) {
		t.Error("VerifyHMAC accepted a signature produced under a different key")
	}
	if VerifyHMAC(key, "message", sig+"x") {
		t.Error("VerifyHMAC accepted a corrupted signature")
	}
}

func TestGCMOpen_RejectsShortCiphertext(t *testing.T) {
	key := testMasterKey(t, 1)
	if _, err := gcmOpen(key, []byte("too short")); err == nil {
		t.Error("gcmOpen accepted a blob shorter than the GCM nonce")
	}
}

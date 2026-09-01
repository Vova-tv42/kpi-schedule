package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func randKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestSealOpenRoundTrip(t *testing.T) {
	key := randKey(t)
	aad := []byte("user-uuid-1")
	plaintext := []byte(`{"PHPSESSID":"abc123","_identity":"xyz"}`)

	ciphertext, err := Seal(key, aad, plaintext)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatalf("ciphertext contains plaintext")
	}

	got, err := Open(key, aad, ciphertext)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("got %q, want %q", got, plaintext)
	}
}

func TestOpenFailsWithWrongAAD(t *testing.T) {
	key := randKey(t)
	ciphertext, err := Seal(key, []byte("user-a"), []byte("secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := Open(key, []byte("user-b"), ciphertext); err == nil {
		t.Fatalf("expected error when AAD does not match (ciphertext replayed to another row)")
	}
}

func TestOpenFailsWithWrongKey(t *testing.T) {
	key1, key2 := randKey(t), randKey(t)
	aad := []byte("user-uuid-1")
	ciphertext, err := Seal(key1, aad, []byte("secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := Open(key2, aad, ciphertext); err == nil {
		t.Fatalf("expected error when key does not match")
	}
}

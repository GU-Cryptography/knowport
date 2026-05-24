package auth

import "testing"

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key, _ := DeriveConfigEncryptionKey("", "test-secret")
	cases := []string{"", "sk-1234567890", "with spaces and 中文", string(make([]byte, 5000))}
	for _, plain := range cases {
		ct, err := EncryptString(key, plain)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		if plain == "" {
			if ct != "" {
				t.Fatalf("empty plaintext must stay empty, got %q", ct)
			}
			continue
		}
		if ct == plain {
			t.Fatalf("ciphertext must differ from plaintext for %q", plain)
		}
		pt, err := DecryptString(key, ct)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if pt != plain {
			t.Fatalf("roundtrip mismatch: got %q want %q", pt, plain)
		}
	}
}

func TestDecryptLegacyPlaintextPassthrough(t *testing.T) {
	key, _ := DeriveConfigEncryptionKey("", "seed")
	// A pre-encryption row: no prefix → returned unchanged.
	out, err := DecryptString(key, "legacy-plain-key")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if out != "legacy-plain-key" {
		t.Fatalf("expected legacy passthrough, got %q", out)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	keyA, _ := DeriveConfigEncryptionKey("", "seed-a")
	keyB, _ := DeriveConfigEncryptionKey("", "seed-b")
	ct, err := EncryptString(keyA, "secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := DecryptString(keyB, ct); err == nil {
		t.Fatal("expected decrypt with wrong key to fail")
	}
}

func TestDeriveKeyFromBase64(t *testing.T) {
	// Valid 32-byte base64 → uses verbatim, not derived.
	raw := "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=" // base64("01234567890123456789012345678901")
	key, derived := DeriveConfigEncryptionKey(raw, "ignored")
	if derived {
		t.Fatal("valid base64 should not be marked derived")
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}

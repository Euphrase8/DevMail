package secretbox

import "testing"

func TestSealOpenRoundTrip(t *testing.T) {
	secret := "cf-super-secret-token-value"
	sealed, err := Seal("passphrase-one", secret)
	if err != nil {
		t.Fatalf("Seal returned error: %v", err)
	}
	if sealed == "" || sealed == secret {
		t.Fatalf("sealed value should be non-empty and not equal to plaintext")
	}
	got, err := Open("passphrase-one", sealed)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if got != secret {
		t.Fatalf("Open = %q, want %q", got, secret)
	}
}

func TestOpenWithWrongKeyFails(t *testing.T) {
	sealed, err := Seal("key-a", "value")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := Open("key-b", sealed); err == nil {
		t.Fatal("expected decryption with a different key to fail")
	}
}

func TestOpenRejectsTampered(t *testing.T) {
	sealed, err := Seal("k", "value")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := Open("k", sealed[:len(sealed)-2]+"AA"); err == nil {
		t.Fatal("expected tampered value to fail")
	}
}

func TestEmptyIsPassThrough(t *testing.T) {
	if s, err := Seal("k", ""); err != nil || s != "" {
		t.Fatalf("Seal empty = %q, %v", s, err)
	}
	if v, err := Open("k", ""); err != nil || v != "" {
		t.Fatalf("Open empty = %q, %v", v, err)
	}
}

func TestMaskedNeverRevealsPrefix(t *testing.T) {
	if got := Masked("abcdef123456"); got != "••••3456" {
		t.Fatalf("Masked = %q", got)
	}
	if got := Masked(""); got != "" {
		t.Fatalf("Masked empty = %q", got)
	}
}

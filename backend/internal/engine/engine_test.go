package engine

import (
	"context"
	"strings"
	"testing"
)

func TestNoopIsInert(t *testing.T) {
	var e Engine = Noop{}
	if e.Configured() {
		t.Fatal("noop engine must not report configured")
	}
	if e.Name() != "none" {
		t.Fatalf("noop engine name = %q, want none", e.Name())
	}
	ctx := context.Background()
	if err := e.EnsureDomain(ctx, "example.com"); err != nil {
		t.Fatal(err)
	}
	if err := e.RemoveDomain(ctx, "example.com"); err != nil {
		t.Fatal(err)
	}
	if err := e.CreateMailbox(ctx, "example.com", "alice", "hash", 0); err != nil {
		t.Fatal(err)
	}
	if err := e.SetPassword(ctx, "example.com", "alice", "hash2"); err != nil {
		t.Fatal(err)
	}
	if err := e.DeleteMailbox(ctx, "example.com", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := e.CreateAlias(ctx, "sales@example.com", "alice@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := e.DeleteAlias(ctx, "sales@example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestHashPasswordUsesArgon2id(t *testing.T) {
	h, err := HashPassword(context.Background(), "Str0ng#Passw0rd")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "argon2id$") {
		t.Fatalf("hash %q is not argon2id-encoded", h)
	}
	if strings.Contains(h, "Str0ng#Passw0rd") {
		t.Fatal("hash must not contain the plaintext password")
	}
}

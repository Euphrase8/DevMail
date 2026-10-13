package auth

import (
	"testing"
	"time"
)

func TestRFC6238Vector(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if got := TOTP(secret, 59/30); got != "287082" {
		t.Fatalf("RFC vector: %s", got)
	}
	if _, ok := matchingStep(secret, "287082", time.Unix(59, 0)); !ok {
		t.Fatal("valid TOTP rejected")
	}
	if _, ok := matchingStep(secret, "000000", time.Unix(59, 0)); ok {
		t.Fatal("invalid code accepted")
	}
}
func TestCSRFTiedToSession(t *testing.T) {
	if csrfSignature("key", "a", "seed") == csrfSignature("key", "b", "seed") {
		t.Fatal("CSRF must bind to session")
	}
}

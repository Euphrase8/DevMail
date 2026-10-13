package auth

import (
	"context"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword(context.Background(), "Str0ng#Passw0rd")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !VerifyPassword(hash, "Str0ng#Passw0rd") {
		t.Fatal("expected correct password to verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("expected wrong password to fail")
	}
	if VerifyPassword("not-a-hash", "Str0ng#Passw0rd") {
		t.Fatal("expected malformed hash to fail")
	}
}

func TestVerifyPasswordRejectsTamperedFields(t *testing.T) {
	hash, err := HashPassword(context.Background(), "another-Str0ng-pw")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if VerifyPassword(hash[:len(hash)-1]+"x", "another-Str0ng-pw") {
		t.Fatal("expected tampered hash to fail")
	}
}

func TestRoleAllows(t *testing.T) {
	cases := []struct {
		role Role
		min  Role
		want bool
	}{
		{RoleSuperAdmin, RoleSuperAdmin, true},
		{RoleSuperAdmin, RoleOperator, true},
		{RoleOperator, RoleSuperAdmin, false},
		{RoleDomainAdmin, RoleServerAdmin, false},
		{RoleServerAdmin, RoleDomainAdmin, true},
	}
	for _, c := range cases {
		if got := c.role.Allowed(c.min); got != c.want {
			t.Errorf("%s.Allowed(%s) = %v, want %v", c.role, c.min, got, c.want)
		}
	}
}

func TestSessionTokenHashing(t *testing.T) {
	token, err := NewSessionToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if HashSessionToken(token) == token {
		t.Fatal("expected stored hash to differ from raw token")
	}
	if HashSessionToken(token) != HashSessionToken(token) {
		t.Fatal("expected stable hashing")
	}
}

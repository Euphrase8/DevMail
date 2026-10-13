package providers

import "testing"

func TestValidateProvider(t *testing.T) {
	good := &providerRequest{Name: "Cloudflare", Kind: "cloudflare", Zone: "example.com"}
	if fields := validateProvider(good, true); len(fields) != 0 {
		t.Fatalf("expected no fields, got %v", fields)
	}

	missingName := &providerRequest{Kind: "manual"}
	if fields := validateProvider(missingName, true); fields["name"] == nil {
		t.Fatal("expected name validation error")
	}

	badKind := &providerRequest{Name: "x", Kind: "bogus"}
	if fields := validateProvider(badKind, true); fields["kind"] == nil {
		t.Fatal("expected kind validation error")
	}

	noZone := &providerRequest{Name: "cf", Kind: "cloudflare"}
	if fields := validateProvider(noZone, true); fields["zone"] == nil {
		t.Fatal("expected zone validation error for API provider")
	}

	manualNoZone := &providerRequest{Name: "manual", Kind: "manual"}
	if fields := validateProvider(manualNoZone, true); len(fields) != 0 {
		t.Fatalf("manual provider should not require a zone: %v", fields)
	}
}

func TestIsHex32(t *testing.T) {
	if !isHex32("0123456789abcdef0123456789abcdef") {
		t.Fatal("valid 32-char hex id rejected")
	}
	if isHex32("example.com") || isHex32("xyz") {
		t.Fatal("non-hex-32 accepted")
	}
}

func TestInitialStatus(t *testing.T) {
	if got := initialStatus("manual", ""); got != "connected" {
		t.Fatalf("manual status = %q", got)
	}
	if got := initialStatus("cloudflare", "token"); got != "pending" {
		t.Fatalf("api status with token = %q", got)
	}
	if got := initialStatus("cloudflare", ""); got != "pending" {
		t.Fatalf("api status without token = %q", got)
	}
}

func TestManualAdapterApply(t *testing.T) {
	res, err := manualAdapter{}.Apply(nil, []Record{{Type: "MX", Name: "example.com"}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res) != 1 || res[0].Status != "manual" {
		t.Fatalf("expected one manual result, got %+v", res)
	}
}

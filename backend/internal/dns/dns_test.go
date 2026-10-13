package dns

import "testing"

func find(records []Record, purpose string) *Record {
	for i := range records {
		if records[i].Purpose == purpose {
			return &records[i]
		}
	}
	return nil
}

func TestPlanGeneratesCoreRecords(t *testing.T) {
	facts := &domainFacts{name: "example.com", verificationToken: "abc123"}
	srv := serverFacts{hostname: "mail.example.com", ipv4: "203.0.113.10"}
	records := plan(facts, srv)

	if r := find(records, "ownership"); r == nil || r.Value != "dmverify=abc123" || r.Name != "_devmail-verification.example.com" {
		t.Fatalf("ownership record wrong: %+v", r)
	}
	if r := find(records, "mx"); r == nil || r.Value != "mail.example.com" || r.Priority != 10 || r.Name != "example.com" {
		t.Fatalf("mx record wrong: %+v", r)
	}
	if r := find(records, "spf"); r == nil || r.Value != "v=spf1 mx -all" {
		t.Fatalf("spf record wrong: %+v", r)
	}
	if r := find(records, "dmarc"); r == nil || r.Name != "_dmarc.example.com" {
		t.Fatalf("dmarc record wrong: %+v", r)
	}
	if r := find(records, "mail-address"); r == nil || r.Value != "203.0.113.10" {
		t.Fatalf("A record wrong: %+v", r)
	}
	if find(records, "dkim") != nil {
		t.Fatal("dkim record should be absent without a public key")
	}
}

func TestPlanFallsBackToMailHostAndAddsDKIM(t *testing.T) {
	facts := &domainFacts{name: "example.org", verificationToken: "t", dkimSelector: "sel", dkimPublicKey: "PUBKEY"}
	records := plan(facts, serverFacts{})

	if r := find(records, "mx"); r == nil || r.Value != "mail.example.org" {
		t.Fatalf("mx fallback wrong: %+v", r)
	}
	if r := find(records, "dkim"); r == nil || r.Name != "sel._domainkey.example.org" || r.Value != "v=DKIM1; k=rsa; p=PUBKEY" {
		t.Fatalf("dkim record wrong: %+v", r)
	}
	if find(records, "mail-address") != nil {
		t.Fatal("A record should be absent without a server ipv4")
	}
}

func TestValidSelector(t *testing.T) {
	for _, ok := range []string{"default", "sel-1", "DKIM2024"} {
		if !validSelector(ok) {
			t.Fatalf("validSelector(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "has space", "a.b", "semi;colon"} {
		if validSelector(bad) {
			t.Fatalf("validSelector(%q) = true, want false", bad)
		}
	}
}

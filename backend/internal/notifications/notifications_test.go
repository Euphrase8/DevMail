package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookNetworkPolicyAndDelivery(t *testing.T) {
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		if r.Method != "POST" {
			t.Error("expected POST")
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	if err := (Webhook{}).Dispatch(context.Background(), server.URL, []byte(`{}`)); err == nil || received {
		t.Fatal("private endpoint must be blocked before dispatch")
	}
	if err := (Webhook{AllowPrivate: true}).Dispatch(context.Background(), server.URL, []byte(`{}`)); err != nil || !received {
		t.Fatalf("configured policy should deliver: %v", err)
	}
}
func TestWebhookRedirectIsRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "http://169.254.169.254/", 302) }))
	defer server.Close()
	if err := (Webhook{AllowPrivate: true}).Dispatch(context.Background(), server.URL, nil); err == nil {
		t.Fatal("redirect must not succeed")
	}
}
func TestValidateRejectsCredentialURL(t *testing.T) {
	if Validate(map[string]any{"url": "https://user:password@example.com"}) == nil {
		t.Fatal("userinfo forbidden")
	}
}

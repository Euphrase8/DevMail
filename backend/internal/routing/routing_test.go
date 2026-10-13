package routing

import "testing"

func TestValidateRouting(t *testing.T) {
	for _, m := range []map[string]any{{"source": "bad", "destination": "b@example.com"}, {"source": "a@example.com", "destination": "a@example.com"}} {
		if Validate(m) == nil {
			t.Fatal("invalid routing accepted")
		}
	}
	if err := Validate(map[string]any{"source": "a@example.com", "destination": "b@example.com"}); err != nil {
		t.Fatal(err)
	}
}

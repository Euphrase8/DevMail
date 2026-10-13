package jobs

import (
	"context"
	"log/slog"
	"testing"
)

func TestRunDispatchesToRegisteredConsumer(t *testing.T) {
	s := NewService(nil, slog.Default())
	s.Register("mailbox.create", func(_ context.Context, payload []byte) (any, error) {
		if string(payload) != `{"mailboxId":"abc"}` {
			t.Fatalf("unexpected payload %s", payload)
		}
		return map[string]any{"address": "alice@example.com"}, nil
	})
	res, err := s.run(context.Background(), "mailbox.create", []byte(`{"mailboxId":"abc"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected a consumer result")
	}
}

func TestRunWithoutConsumerFails(t *testing.T) {
	s := NewService(nil, slog.Default())
	res, err := s.run(context.Background(), "mailbox.create", nil)
	if err == nil {
		t.Fatal("unbound kind must not report success")
	}
	if res != nil {
		t.Fatalf("expected nil result, got %v", res)
	}
}

func TestRunUnknownKindErrors(t *testing.T) {
	s := NewService(nil, slog.Default())
	if _, err := s.run(context.Background(), "not.a.kind", nil); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

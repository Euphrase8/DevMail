package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/config"
	"github.com/Euphrase8/DevMail/backend/internal/db"
	"github.com/Euphrase8/DevMail/backend/internal/engine"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProvisioningIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL and HTTP integration checks")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "verify_" + time.Now().Format("20060102150405")
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Options{Pool: pool, Context: ctx, Config: &config.Config{SecretKey: "integration-only", Env: "development"}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), MailEngine: engine.NewSQL(pool)})
	server := httptest.NewServer(router)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	call := func(method, path, body string, want int) map[string]any {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+"/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, res.StatusCode, want, raw)
		}
		var envelope struct{ Data map[string]any }
		_ = json.Unmarshal(raw, &envelope)
		return envelope.Data
	}
	call("GET", "/mailboxes/engine", "", 401)
	credentials := `{"email":"admin@example.test","name":"Admin","password":"Strong-test-password-123"}`
	call("POST", "/auth/setup", credentials, 201)
	call("POST", "/auth/login", credentials, 200)
	domain := call("POST", "/domains", `{"name":"integration.example.test"}`, 201)["id"].(string)
	mailbox := call("POST", "/domains/"+domain+"/mailboxes", `{"local":"alice","password":"Strong-test-password-123","quotaBytes":1024}`, 201)["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for {
		state := call("GET", "/mailboxes/"+mailbox+"/provisioning", "", 200)
		if state["status"] == "managed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not provision mailbox")
		}
		time.Sleep(50 * time.Millisecond)
	}
	var hash string
	var quota int64
	if err = pool.QueryRow(ctx, `SELECT password_hash,quota_bytes FROM engine_mailboxes WHERE email='alice@integration.example.test'`).Scan(&hash, &quota); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "{ARGON2ID}$argon2id$") || quota != 1024 {
		t.Fatal("engine did not store compatible hash and quota")
	}
	call("PATCH", "/mailboxes/"+mailbox, `{"displayName":"Alice","quotaBytes":2048}`, 200)
	call("DELETE", "/mailboxes/"+mailbox, "", 200)
	deadline = time.Now().Add(8 * time.Second)
	for {
		var enabled bool
		err = pool.QueryRow(ctx, `SELECT enabled FROM engine_mailboxes WHERE email='alice@integration.example.test'`).Scan(&enabled)
		if err != nil {
			t.Fatal(err)
		}
		if !enabled {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("teardown not applied")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

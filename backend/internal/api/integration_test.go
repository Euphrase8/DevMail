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

	"github.com/Euphrase8/DevMail/backend/internal/auth"
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
		if method != "GET" && method != "HEAD" {
			csrfRes, err := client.Get(server.URL + "/api/v1/auth/csrf")
			if err != nil {
				t.Fatal(err)
			}
			var csrf struct{ Data struct{ Token string } }
			if json.NewDecoder(csrfRes.Body).Decode(&csrf) != nil {
				t.Fatal("CSRF fetch failed")
			}
			csrfRes.Body.Close()
			req.Header.Set("X-CSRF-Token", csrf.Data.Token)
		}
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
	call("PATCH", "/settings", `{"name":"Verified workspace","timezone":"UTC","webmail":"https://webmail.example.test","mailEngine":"sql","defaultQuotaBytes":2048}`, 200)
	call("PATCH", "/settings", `{"provisioning_enabled":false}`, 422)
	srv := call("POST", "/servers", `{"name":"Local blocked server","hostname":"localhost","ipv4":"127.0.0.1","environment":"development"}`, 201)["id"].(string)
	call("POST", "/servers/"+srv+"/maintenance", `{"enabled":true}`, 200)
	call("POST", "/servers/"+srv+"/check", "", 200)
	call("PATCH", "/servers/"+srv, `{"name":"Updated server","hostname":"localhost","ipv4":"127.0.0.1","environment":"development"}`, 200)
	call("DELETE", "/servers/"+srv, "", 200)
	rule := call("POST", "/routing-rules", `{"name":"Forward","enabled":true,"config":{"source":"a@example.test","destination":"b@example.test"}}`, 201)["id"].(string)
	call("PATCH", "/routing-rules/"+rule, `{"name":"Updated","enabled":false,"config":{"source":"a@example.test","destination":"c@example.test"}}`, 200)
	call("DELETE", "/routing-rules/"+rule, "", 200)
	backup := call("POST", "/backups", `{"name":"Database","enabled":true,"config":{"kind":"postgres"}}`, 201)["id"].(string)
	result := call("POST", "/backups/"+backup+"/run", "", 200)
	if result["status"] != "failed" {
		t.Fatal("unconfigured backup claimed success")
	}
	channel := call("POST", "/notifications", `{"name":"Webhook","enabled":true,"config":{"url":"http://127.0.0.1:1/private-secret"}}`, 201)["id"].(string)
	if result = call("POST", "/notifications/"+channel+"/test", "", 200); result["status"] != "failed" {
		t.Fatal("SSRF endpoint accepted")
	}
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
	operator := call("POST", "/users", `{"email":"operator@example.test","name":"Operator","role":"operator","password":"Strong-operator-password-123"}`, 201)["id"].(string)
	call("POST", "/auth/login", `{"email":"operator@example.test","password":"Strong-operator-password-123"}`, 200)
	call("POST", "/servers", `{}`, 403)
	call("GET", "/users", "", 403)
	call("POST", "/auth/login", credentials, 200)
	call("DELETE", "/users/"+operator, "", 200)
	// Missing CSRF must fail even with a valid session.
	request, _ := http.NewRequest("POST", server.URL+"/api/v1/auth/rotate", nil)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("mutation without CSRF accepted")
	}
	enroll := call("POST", "/auth/totp/enroll", `{"password":"Strong-test-password-123"}`, 200)
	secret := enroll["secret"].(string)
	code := auth.TOTP(secret, time.Now().Unix()/30)
	verify := call("POST", "/auth/totp/verify", `{"code":"`+code+`"}`, 200)
	recovery := verify["recoveryCodes"].([]any)[0].(string)
	call("POST", "/auth/logout", "", 200)
	call("POST", "/auth/login", credentials, 401)
	call("POST", "/auth/login", `{"email":"admin@example.test","password":"Strong-test-password-123","code":"`+recovery+`"}`, 200)
	call("POST", "/auth/login", `{"email":"admin@example.test","password":"Strong-test-password-123","code":"`+recovery+`"}`, 401)
	call("POST", "/auth/rotate", "", 200)
	call("POST", "/auth/password", `{"currentPassword":"Strong-test-password-123","newPassword":"Changed-strong-password-456"}`, 200)
	call("GET", "/auth/me", "", 401)
	for i := 0; i < 5; i++ {
		call("POST", "/auth/login", `{"email":"admin@example.test","password":"bad"}`, 401)
	}
	var locked bool
	if err = pool.QueryRow(ctx, `SELECT locked_until>now() FROM auth_attempts WHERE key=$1`, "account:"+auth.HashSessionToken("admin@example.test")).Scan(&locked); err != nil || !locked {
		t.Fatal("persistent account lockout missing")
	}
	var loginEvents int
	if err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE action IN ('auth.login','auth.login_failed','auth.logout')`).Scan(&loginEvents); err != nil || loginEvents < 3 {
		t.Fatal("auth audit events missing")
	}

}

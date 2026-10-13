// devmail-tools performs operational commands that keep the real mail stack in
// sync with the managed records and export materials the stack needs.
//
//   - resync:      reconciles the engine_* tables from the managed records and
//     marks them provisioned. Safe to re-run at any time.
//   - dkim-export: writes decrypted DKIM private keys (requires SECRET_KEY)
//     plus the OpenDKIM KeyTable/SigningTable files.
//
// The binary never reads passwords or writes secrets to stdout; DKIM private
// keys are written only to the requested directory with 0600 permissions.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/secretbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultDSN = "postgres://postgres:1508@127.0.0.1:5432/devmail?sslmode=disable"

func usage() {
	fmt.Fprintf(os.Stderr, `devmail-tools - operational commands for the DevMail mail engine.

Usage:
  devmail-tools resync
        Reconcile the engine_* tables from the managed records and mark them
        provisioned. Safe to re-run at any time.
  devmail-tools dkim-export --dir <path>
        Export decrypted DKIM private keys plus the OpenDKIM KeyTable and
        SigningTable files into <path>.

Environment:
  DATABASE_URL   PostgreSQL connection string (default: local dev database).
  SECRET_KEY     Passphrase that encrypts sensitive values at rest; required
                 for dkim-export.
`)
	os.Exit(2)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
	}
	switch args[0] {
	case "resync":
		if err := cmdResync(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "resync:", err)
			os.Exit(1)
		}
	case "dkim-export":
		dir := ""
		fs := flag.NewFlagSet("dkim-export", flag.ExitOnError)
		fs.StringVar(&dir, "dir", "", "output directory for keys and table files (required)")
		_ = fs.Parse(args[1:])
		if dir == "" {
			fs.Usage()
			os.Exit(2)
		}
		if err := cmdDkimExport(ctx, dir); err != nil {
			fmt.Fprintln(os.Stderr, "dkim-export:", err)
			os.Exit(1)
		}
	default:
		usage()
	}
}

func openPool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultDSN
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", redactDSN(dsn), err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// redactDSN hides the password from connection error messages.
func redactDSN(dsn string) string {
	out := ""
	for i := 0; i < len(dsn); i++ {
		if dsn[i] == ':' {
			for j := i + 1; j < len(dsn); j++ {
				if dsn[j] == '@' {
					out += ":***@"
					i = j
					goto next
				}
			}
		}
		out += string(dsn[i])
	next:
	}
	return out
}

// cmdResync rebuilds the engine_* tables from the managed records in one
// transaction and marks the touched records as provisioned.
func cmdResync(ctx context.Context) error {
	pool, err := openPool(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	domainsAdded, err := tx.Exec(ctx, `
		INSERT INTO engine_domains (domain, enabled, updated_at)
		SELECT name, true, now() FROM domains WHERE deleted_at IS NULL
		ON CONFLICT (domain) DO UPDATE SET enabled=true, deleted_at=NULL, updated_at=now()`)
	if err != nil {
		return fmt.Errorf("sync engine_domains: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE engine_domains SET enabled=false, deleted_at=now(), updated_at=now()
		WHERE deleted_at IS NULL AND domain NOT IN (SELECT name FROM domains WHERE deleted_at IS NULL)`); err != nil {
		return fmt.Errorf("disable stale engine_domains: %w", err)
	}

	mailboxesSynced, err := tx.Exec(ctx, `
		INSERT INTO engine_mailboxes (email, domain, local, password_hash, quota_bytes, enabled, updated_at)
		SELECT mb.local || '@' || d.name, d.name, mb.local,
		       '{ARGON2ID}$' || mb.password_hash, mb.quota_bytes, mb.status = 'active', now()
		FROM mailboxes mb
		JOIN domains d ON d.id = mb.domain_id
		WHERE mb.deleted_at IS NULL AND mb.password_hash <> ''
		ON CONFLICT (email) DO UPDATE SET domain=EXCLUDED.domain, local=EXCLUDED.local,
			password_hash=EXCLUDED.password_hash, quota_bytes=EXCLUDED.quota_bytes,
			enabled=EXCLUDED.enabled, deleted_at=NULL, updated_at=now()`)
	if err != nil {
		return fmt.Errorf("sync engine_mailboxes: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE engine_mailboxes SET enabled=false, deleted_at=now(), updated_at=now()
		WHERE deleted_at IS NULL AND email NOT IN (
			SELECT mb.local || '@' || d.name
			FROM mailboxes mb JOIN domains d ON d.id = mb.domain_id
			WHERE mb.deleted_at IS NULL AND mb.password_hash <> '')`); err != nil {
		return fmt.Errorf("disable stale engine_mailboxes: %w", err)
	}

	aliasesSynced, err := tx.Exec(ctx, `
		INSERT INTO engine_aliases (source, destination, enabled, updated_at)
		SELECT al.source || '@' || d.name, al.destination, al.enabled, now()
		FROM aliases al JOIN domains d ON d.id = al.domain_id
		WHERE al.deleted_at IS NULL AND al.enabled
		ON CONFLICT (source) DO UPDATE SET destination=EXCLUDED.destination,
			enabled=EXCLUDED.enabled, deleted_at=NULL, updated_at=now()`)
	if err != nil {
		return fmt.Errorf("sync engine_aliases: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE engine_aliases SET enabled=false, deleted_at=now(), updated_at=now()
		WHERE deleted_at IS NULL AND source NOT IN (
			SELECT al.source || '@' || d.name
			FROM aliases al JOIN domains d ON d.id = al.domain_id
			WHERE al.deleted_at IS NULL AND al.enabled)`); err != nil {
		return fmt.Errorf("disable stale engine_aliases: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE mailboxes SET engine_status='managed', provisioning_error='', last_provisioned_at=now()
		WHERE deleted_at IS NULL AND password_hash <> '' AND status='active'`); err != nil {
		return fmt.Errorf("mark mailboxes provisioned: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE aliases SET last_provisioned_at=now()
		WHERE deleted_at IS NULL AND enabled`); err != nil {
		return fmt.Errorf("mark aliases provisioned: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE domains SET provisioning_error='', last_provisioned_at=now()
		WHERE deleted_at IS NULL`); err != nil {
		return fmt.Errorf("mark domains provisioned: %w", err)
	}

	// The worker gate must be on so future mutations queue correctly.
	if _, err := tx.Exec(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES ('provisioning_enabled', 'true', now())
		ON CONFLICT (key) DO UPDATE SET value='true', updated_at=now()`); err != nil {
		return fmt.Errorf("enable provisioning: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Printf("resync complete\n  engine_domains touched:   %d\n  engine_mailboxes touched: %d\n  engine_aliases touched:   %d\n  provisioning_expired:     on\n",
		domainsAdded.RowsAffected(), mailboxesSynced.RowsAffected(), aliasesSynced.RowsAffected())
	return nil
}

type dkimEntry struct {
	domain   string
	selector string
	path     string
}

// cmdDkimExport decrypts stored DKIM private keys and writes them alongside the
// OpenDKIM KeyTable and SigningTable files.
func cmdDkimExport(ctx context.Context, dir string) error {
	passphrase := os.Getenv("SECRET_KEY")
	if passphrase == "" {
		return errors.New("SECRET_KEY is required to decrypt DKIM keys")
	}
	pool, err := openPool(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `
		SELECT name, dkim_selector, dkim_private_key
		FROM domains WHERE deleted_at IS NULL AND dkim_selector <> '' AND dkim_private_key <> ''`)
	if err != nil {
		return fmt.Errorf("query domains: %w", err)
	}
	defer rows.Close()

	var entries []dkimEntry
	for rows.Next() {
		var domain, selector, sealed string
		if err := rows.Scan(&domain, &selector, &sealed); err != nil {
			return fmt.Errorf("scan domain row: %w", err)
		}
		pem, err := secretbox.Open(passphrase, sealed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: could not decrypt DKIM key (%v)\n", domain, err)
			continue
		}
		if pem == "" {
			continue
		}
		sub := filepath.Join(dir, domain)
		if err := os.MkdirAll(sub, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", sub, err)
		}
		keyPath := filepath.Join(sub, selector+".private")
		if err := os.WriteFile(keyPath, []byte(pem), 0o600); err != nil {
			return fmt.Errorf("write key: %w", err)
		}
		entries = append(entries, dkimEntry{domain: domain, selector: selector, path: keyPath})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read domains: %w", err)
	}

	keyTable := "# DevMail OpenDKIM key table (generated by devmail-tools dkim-export)\n"
	signingTable := "# DevMail OpenDKIM signing table (generated by devmail-tools dkim-export)\n"
	for _, e := range entries {
		keyTable += fmt.Sprintf("%s._domainkey.%s %s:%s:%s\n", e.selector, e.domain, e.domain, e.selector, e.path)
		signingTable += fmt.Sprintf("*@%s %s._domainkey.%s\n", e.domain, e.selector, e.domain)
	}
	if err := os.WriteFile(filepath.Join(dir, "KeyTable"), []byte(keyTable), 0o600); err != nil {
		return fmt.Errorf("write KeyTable: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SigningTable"), []byte(signingTable), 0o600); err != nil {
		return fmt.Errorf("write SigningTable: %w", err)
	}

	fmt.Printf("dkim-export complete: %d key(s) written to %s\n", len(entries), dir)
	return nil
}

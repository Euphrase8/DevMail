// Package engine defines the pluggable mail-engine boundary used for
// provisioning. The default adapter is a no-op (records stay "unmanaged");
// the SQL adapter writes a Dovecot/Postfix-style virtual-users schema that a
// real mail server can consume directly. No adapter shells out or executes
// user-supplied commands.
package engine

import (
	"context"
	"strings"

	"github.com/Euphrase8/DevMail/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Engine provisions domains, mailboxes and aliases on a mail backend.
type Engine interface {
	// Configured reports whether a live engine is available. When false the
	// API still manages records but reports them as "unmanaged".
	Configured() bool
	// Name identifies the adapter for status reporting.
	Name() string
	// EnsureDomain creates or re-enables a domain on the engine.
	EnsureDomain(ctx context.Context, domain string) error
	// RemoveDomain disables a domain on the engine.
	RemoveDomain(ctx context.Context, domain string) error
	// CreateMailbox creates a mailbox with an already-hashed password.
	CreateMailbox(ctx context.Context, domain, local, passwordHash string, quotaBytes int64) error
	// SetPassword replaces a mailbox password hash.
	SetPassword(ctx context.Context, domain, local, passwordHash string) error
	// DeleteMailbox removes a mailbox.
	DeleteMailbox(ctx context.Context, domain, local string) error
	// CreateAlias creates a forwarding alias.
	CreateAlias(ctx context.Context, source, destination string) error
	// DeleteAlias removes a forwarding alias.
	DeleteAlias(ctx context.Context, source string) error
}

// Noop is the default adapter used until a real engine is configured.
type Noop struct{}

// Configured always reports false.
func (Noop) Configured() bool { return false }

// Name identifies the no-op adapter.
func (Noop) Name() string { return "none" }

func (Noop) EnsureDomain(context.Context, string) error                         { return nil }
func (Noop) RemoveDomain(context.Context, string) error                         { return nil }
func (Noop) CreateMailbox(context.Context, string, string, string, int64) error { return nil }
func (Noop) SetPassword(context.Context, string, string, string) error          { return nil }
func (Noop) DeleteMailbox(context.Context, string, string) error                { return nil }
func (Noop) CreateAlias(context.Context, string, string) error                  { return nil }
func (Noop) DeleteAlias(context.Context, string) error                          { return nil }

// SQL adapts the managed records into the engine_* tables. Passwords are
// stored as Argon2id hashes; plaintext never reaches the engine.
type SQL struct {
	pool *pgxpool.Pool
}

// NewSQL returns a SQL-backed engine adapter.
func NewSQL(pool *pgxpool.Pool) *SQL { return &SQL{pool: pool} }

// Configured always reports true for the SQL adapter.
func (s *SQL) Configured() bool { return true }

// Name identifies the SQL adapter.
func (s *SQL) Name() string { return "sql" }

func normalize(domain string) string { return strings.ToLower(strings.TrimSpace(domain)) }

// EnsureDomain upserts an enabled domain row.
func (s *SQL) EnsureDomain(ctx context.Context, domain string) error {
	domain = normalize(domain)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO engine_domains (domain, enabled, updated_at) VALUES ($1, true, now())
		ON CONFLICT (domain) DO UPDATE SET enabled=true, deleted_at=NULL, updated_at=now()`, domain)
	return err
}

// RemoveDomain soft-disables a domain row.
func (s *SQL) RemoveDomain(ctx context.Context, domain string) error {
	domain = normalize(domain)
	_, err := s.pool.Exec(ctx,
		`UPDATE engine_domains SET enabled=false, deleted_at=now(), updated_at=now() WHERE domain=$1`, domain)
	return err
}

// CreateMailbox upserts a mailbox row keyed by the full address.
func (s *SQL) CreateMailbox(ctx context.Context, domain, local, passwordHash string, quotaBytes int64) error {
	domain = normalize(domain)
	email := local + "@" + domain
	if strings.HasPrefix(passwordHash, "argon2id$") {
		passwordHash = "{ARGON2ID}$" + passwordHash
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO engine_mailboxes (email, domain, local, password_hash, quota_bytes, enabled, updated_at)
		VALUES ($1,$2,$3,$4,$5,true,now())
		ON CONFLICT (email) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			quota_bytes = EXCLUDED.quota_bytes,
			enabled = true, deleted_at=NULL, updated_at=now()`,
		email, domain, local, passwordHash, quotaBytes)
	return err
}

// SetPassword updates only the password hash for an existing mailbox.
func (s *SQL) SetPassword(ctx context.Context, domain, local, passwordHash string) error {
	email := local + "@" + normalize(domain)
	if strings.HasPrefix(passwordHash, "argon2id$") {
		passwordHash = "{ARGON2ID}$" + passwordHash
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE engine_mailboxes SET password_hash=$2, updated_at=now() WHERE email=$1`, email, passwordHash)
	return err
}

// DeleteMailbox soft-disables a mailbox row.
func (s *SQL) DeleteMailbox(ctx context.Context, domain, local string) error {
	email := local + "@" + normalize(domain)
	_, err := s.pool.Exec(ctx,
		`UPDATE engine_mailboxes SET enabled=false, deleted_at=now(), updated_at=now() WHERE email=$1`, email)
	return err
}

// CreateAlias upserts an alias row.
func (s *SQL) CreateAlias(ctx context.Context, source, destination string) error {
	source = strings.ToLower(strings.TrimSpace(source))
	_, err := s.pool.Exec(ctx, `
		INSERT INTO engine_aliases (source, destination, enabled, updated_at) VALUES ($1,$2,true,now())
		ON CONFLICT (source) DO UPDATE SET destination=EXCLUDED.destination, enabled=true, deleted_at=NULL, updated_at=now()`,
		source, destination)
	return err
}

// DeleteAlias soft-disables an alias row.
func (s *SQL) DeleteAlias(ctx context.Context, source string) error {
	source = strings.ToLower(strings.TrimSpace(source))
	_, err := s.pool.Exec(ctx,
		`UPDATE engine_aliases SET enabled=false, deleted_at=now(), updated_at=now() WHERE source=$1`, source)
	return err
}

// HashPassword exposes the shared Argon2id hasher so record writers hash once
// and the same hash is reused by the engine (plaintext is never queued).
func HashPassword(ctx context.Context, password string) (string, error) {
	return auth.HashPassword(ctx, password)
}

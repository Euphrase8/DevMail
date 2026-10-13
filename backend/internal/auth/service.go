package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionCookie is the name of the authentication cookie.
const SessionCookie = "dm_session"

// SessionLifetime is how long a session token remains valid.
const SessionLifetime = 24 * time.Hour

// ErrNotFound is returned when a resource does not exist.
var ErrNotFound = errors.New("not found")

// User is the signed-in administrator identity attached to requests.
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      Role   `json:"role"`
	IsActive  bool   `json:"isActive"`
	CreatedAt string `json:"createdAt"`
}

// Service implements authentication and session persistence.
type Service struct {
	pool *pgxpool.Pool
}

// NewService creates an auth service backed by the given pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// UserCount returns the number of active users (used to detect bootstrap state).
func (s *Service) UserCount(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE is_active = true").Scan(&n)
	return n, err
}

const userColumns = `id::text, email, name, role, is_active, to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	var role string
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &role, &u.IsActive, &u.CreatedAt); err != nil {
		return nil, err
	}
	parsed, ok := ParseRole(role)
	if !ok {
		return nil, errors.New("invalid stored role: " + role)
	}
	u.Role = parsed
	return &u, nil
}

// Setup creates the initial super administrator when no users exist yet.
func (s *Service) Setup(ctx context.Context, email, name, password string) (*User, error) {
	n, err := s.UserCount(ctx)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, errors.New("bootstrap already completed")
	}
	hash, err := HashPassword(ctx, password)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, name, role, password_hash)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+userColumns,
		email, name, RoleSuperAdmin, hash)
	return scanUser(row)
}

// Login verifies credentials and creates a session when valid.
func (s *Service) Login(ctx context.Context, email, password, ip, userAgent string) (*User, string, error) {
	var id, storedHash string
	var active bool
	var role string
	var name string
	err := s.pool.QueryRow(ctx,
		`SELECT id::text, password_hash, is_active, role::text, name FROM users WHERE lower(email) = lower($1)`,
		email).Scan(&id, &storedHash, &active, &role, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		// Constant-time-ish response: still perform a hash to reduce timing signal.
		_, _ = HashPassword(ctx, "dummy-password-for-timing")
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	if !active {
		return nil, "", errors.New("account is disabled")
	}
	if !VerifyPassword(storedHash, password) {
		return nil, "", errors.New("invalid credentials")
	}
	parsed, _ := ParseRole(role)
	token, err := NewSessionToken()
	if err != nil {
		return nil, "", err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO sessions (user_id, token_hash, ip, user_agent, expires_at)
		 VALUES ($1, $2, $3, $4, now() + $5::interval)`,
		id, HashSessionToken(token), ip, userAgent, "24 hours")
	if err != nil {
		return nil, "", err
	}
	user := &User{ID: id, Email: email, Name: name, Role: parsed, IsActive: active, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	return user, token, nil
}

// UserBySession resolves a user from a session cookie token.
func (s *Service) UserBySession(ctx context.Context, token string) (*User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT u.id::text, u.email, u.name, u.role::text, u.is_active,
		        to_char(u.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.revoked = false AND s.expires_at > now() AND u.is_active = true`,
		HashSessionToken(token))
	return scanUser(row)
}

// Logout revokes the current session.
func (s *Service) Logout(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET revoked = true WHERE token_hash = $1`,
		HashSessionToken(token))
	return err
}

// CreateUser creates a new administrator with the given role.
func (s *Service) CreateUser(ctx context.Context, email, name string, role Role, password string) (*User, error) {
	hash, err := HashPassword(ctx, password)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, name, role, password_hash)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+userColumns,
		email, name, role, hash)
	return scanUser(row)
}

// ListUsers lists all administrators.
func (s *Service) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateUser updates name/role/active status and optionally resets the password.
func (s *Service) UpdateUser(ctx context.Context, id, name string, role Role, active bool, newPassword string) (*User, error) {
	if newPassword != "" {
		hash, err := HashPassword(ctx, newPassword)
		if err != nil {
			return nil, err
		}
		row := s.pool.QueryRow(ctx,
			`UPDATE users SET name = $2, role = $3, is_active = $4, password_hash = $5, updated_at = now()
			 WHERE id = $1 RETURNING `+userColumns,
			id, name, role, active, hash)
		return scanUser(row)
	}
	row := s.pool.QueryRow(ctx,
		`UPDATE users SET name = $2, role = $3, is_active = $4, updated_at = now()
		 WHERE id = $1 RETURNING `+userColumns,
		id, name, role, active)
	return scanUser(row)
}

// DeleteUser removes an administrator by id.
func (s *Service) DeleteUser(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1 AND role <> 'super_admin'`, id)
	return err
}

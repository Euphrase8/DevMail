package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func csrfSignature(key, session, seed string) string {
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(session + "|" + seed))
	return hex.EncodeToString(m.Sum(nil))
}
func CSRF(key string, secure bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, _ := c.Cookie(SessionCookie)
		seed, _ := c.Cookie("dm_csrf_seed")
		if c.Request.Method == "GET" && c.Request.URL.Path == "/api/v1/auth/csrf" {
			if seed == "" {
				seed, _ = NewSessionToken()
			}
			token := csrfSignature(key, session, seed)
			c.SetSameSite(http.SameSiteStrictMode)
			c.SetCookie("dm_csrf_seed", seed, 86400, "/", "", secure, true)
			c.SetCookie("dm_csrf", token, 86400, "/", "", secure, false)
			httpapi.Respond(c, 200, gin.H{"token": token})
			c.Abort()
			return
		}
		if c.Request.Method == "POST" || c.Request.Method == "PATCH" || c.Request.Method == "PUT" || c.Request.Method == "DELETE" {
			cookie, _ := c.Cookie("dm_csrf")
			header := c.GetHeader("X-CSRF-Token")
			expected := csrfSignature(key, session, seed)
			if seed == "" || header == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) != 1 || subtle.ConstantTimeCompare([]byte(expected), []byte(header)) != 1 {
				httpapi.Fail(c, 403, httpapi.ErrForbidden, "CSRF token is missing or invalid.")
				return
			}
		}
		c.Next()
	}
}

// MutationPolicy applies role checks on the server for every protected mutation.
func MutationPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" {
			c.Next()
			return
		}
		path := strings.TrimPrefix(c.Request.URL.Path, "/api/v1/")
		min := RoleDomainAdmin
		switch strings.Split(path, "/")[0] {
		case "auth":
			c.Next()
			return
		case "settings", "users", "backups":
			min = RoleSuperAdmin
		case "servers", "providers", "notifications", "jobs":
			min = RoleServerAdmin
		}
		RequireRole(min)(c)
	}
}

func TOTP(secret string, step int64) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(message[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 15
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1000000)
}
func matchingStep(secret, code string, now time.Time) (int64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	step := now.Unix() / 30
	for delta := int64(-1); delta <= 1; delta++ {
		if subtle.ConstantTimeCompare([]byte(TOTP(secret, step+delta)), []byte(code)) == 1 {
			return step + delta, true
		}
	}
	return 0, false
}

func AuthEvent(ctx context.Context, pool *pgxpool.Pool, user *User, action, ip string) {
	var id any
	email := ""
	if user != nil {
		id = user.ID
		email = user.Email
	}
	_, _ = pool.Exec(ctx, `INSERT INTO audit_log(user_id,actor_email,action,resource_type,resource_id,ip) VALUES($1,$2,$3,'auth','',$4)`, id, email, action, ip)
}
func PersistentRateLimit(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := HashSessionToken(c.ClientIP())
		var attempts int
		err := pool.QueryRow(c.Request.Context(), `INSERT INTO auth_attempts(key) VALUES($1) ON CONFLICT(key) DO UPDATE SET attempts=CASE WHEN auth_attempts.window_start < now()-interval '1 minute' THEN 1 ELSE auth_attempts.attempts+1 END,window_start=CASE WHEN auth_attempts.window_start < now()-interval '1 minute' THEN now() ELSE auth_attempts.window_start END RETURNING attempts`, key).Scan(&attempts)
		if err != nil {
			httpapi.InternalError(c)
			return
		}
		if attempts > 20 {
			c.Header("Retry-After", "60")
			httpapi.Fail(c, 429, httpapi.ErrRateLimited, "Too many authentication attempts.")
			return
		}
		c.Next()
	}
}

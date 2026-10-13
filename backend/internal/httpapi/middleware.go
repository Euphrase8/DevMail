package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "dm.request_id"

// RequestID returns the request id carried in the context, if any.
func RequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("150405")
	}
	return hex.EncodeToString(b)
}

// RequestIDMiddleware attaches a request id and emits structured access logs.
func RequestIDMiddleware(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = newRequestID()
		}
		c.Set(requestIDKey, id)
		c.Writer.Header().Set("X-Request-Id", id)

		c.Next()

		status := c.Writer.Status()
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}
		log.LogAttrs(c.Request.Context(), level, "request",
			slog.String("id", id),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int64("durationMs", time.Since(start).Milliseconds()),
			slog.String("ip", c.ClientIP()),
		)
	}
}

// RecoverMiddleware converts panics into clean 500 responses.
func RecoverMiddleware(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered", slog.Any("panic", rec), slog.String("id", RequestID(c)))
				Fail(c, http.StatusInternalServerError, ErrInternal, "An internal error occurred.")
			}
		}()
		c.Next()
	}
}

// CORSMiddleware allows configured browser origins to call the API.
func CORSMiddleware(allowed []string) gin.HandlerFunc {
	allowedSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		allowedSet[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if allowedSet[origin] {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Headers", "Content-Type, X-Request-Id")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			}
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}

type rateBucket struct {
	tokens  int
	lastRef time.Time
	until   time.Time
}

// rateLimiter is a minimal in-memory token bucket used to throttle
// authentication endpoints.
type rateLimiter struct {
	mu   sync.Mutex
	per  time.Duration
	max  int
	lock time.Duration
	b    map[string]*rateBucket
}

func newRateLimiter(max int, per, lock time.Duration) *rateLimiter {
	return &rateLimiter{per: per, max: max, lock: lock, b: make(map[string]*rateBucket)}
}

func (r *rateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	b, ok := r.b[key]
	if !ok {
		r.b[key] = &rateBucket{tokens: r.max - 1, lastRef: now}
		return true
	}
	if now.Before(b.until) {
		return false
	}
	b.tokens += int(now.Sub(b.lastRef) / r.per)
	if b.tokens > r.max {
		b.tokens = r.max
	}
	b.lastRef = now
	if b.tokens <= 0 {
		b.until = now.Add(r.lock)
		return false
	}
	b.tokens--
	return true
}

// AuthRateLimit protects login/setup against brute force.
func AuthRateLimit(max int, per, lock time.Duration) gin.HandlerFunc {
	rl := newRateLimiter(max, per, lock)
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.Header("Retry-After", "60")
			Fail(c, http.StatusTooManyRequests, ErrRateLimited, "Too many attempts. Try again shortly.")
			return
		}
		c.Next()
	}
}

// SecurityHeadersMiddleware adds baseline security response headers.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}
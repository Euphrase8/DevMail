package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler exposes authentication and user administration HTTP handlers.
type Handler struct {
	svc    *Service
	pool   *pgxpool.Pool
	key    string
	secure bool
}

// NewHandler wires the auth service handlers to a database pool.
func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{svc: NewService(pool), pool: pool}
}

// HasUsers is a simple helper used by the frontend to decide between
// bootstrap and login screens.
func (h *Handler) HasUsers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	n, err := h.svc.UserCount(ctx)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	httpapi.Respond(c, http.StatusOK, gin.H{"hasUsers": n > 0})
}

type setupRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (r *setupRequest) validate() map[string]any {
	fields := map[string]any{}
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	if !validEmail(r.Email) {
		fields["email"] = "A valid administrator email is required."
	}
	if r.Password == "" {
		fields["password"] = "A password is required."
	} else if len(r.Password) < 12 {
		fields["password"] = "Password must be at least 12 characters."
	}
	return fields
}

// Setup handles POST /api/v1/auth/setup.
func (h *Handler) Setup(c *gin.Context) {
	var req setupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	if fields := req.validate(); len(fields) > 0 {
		httpapi.FailFields(c, "Check the highlighted fields and try again.", fields)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	user, err := h.svc.Setup(ctx, req.Email, req.Name, req.Password)
	if err != nil {
		if err.Error() == "bootstrap already completed" {
			httpapi.Fail(c, http.StatusConflict, httpapi.ErrConflict, "An administrator already exists.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	AuthEvent(ctx, h.pool, user, "auth.setup", c.ClientIP())
	httpapi.Respond(c, http.StatusCreated, user)
}

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	user, token, err := h.svc.Login(ctx, req.Email, req.Password, c.ClientIP(), c.Request.UserAgent(), req.Code)
	if err != nil {
		AuthEvent(ctx, h.pool, nil, "auth.login_failed", c.ClientIP())
		if errors.Is(err, ErrAccountLocked) {
			c.Header("Retry-After", "900")
			httpapi.Fail(c, http.StatusTooManyRequests, httpapi.ErrRateLimited, "Too many failed sign-in attempts. This account is temporarily locked — try again in about 15 minutes.")
			return
		}
		if errors.Is(err, ErrNotFound) || err.Error() == "invalid credentials" {
			httpapi.Fail(c, http.StatusUnauthorized, httpapi.ErrUnauthorized, "Invalid email or password.")
			return
		}
		if err.Error() == "account is disabled" {
			httpapi.Fail(c, http.StatusForbidden, httpapi.ErrForbidden, "This account is disabled.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	if old, err := c.Cookie(SessionCookie); err == nil {
		_ = h.svc.Logout(ctx, old)
	}
	h.cookie(c, token, int(SessionLifetime.Seconds()))
	AuthEvent(ctx, h.pool, user, "auth.login", c.ClientIP())
	httpapi.Respond(c, http.StatusOK, user)
}

// Logout handles POST /api/v1/auth/logout.
func (h *Handler) Logout(c *gin.Context) {
	if token, err := c.Cookie(SessionCookie); err == nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		_ = h.svc.Logout(ctx, token)
	}
	h.cookie(c, "", -1)
	AuthEvent(c.Request.Context(), h.pool, CurrentUser(c), "auth.logout", c.ClientIP())
	httpapi.Respond(c, http.StatusOK, gin.H{"ok": true})
}

// Me handles GET /api/v1/auth/me.
func (h *Handler) Me(c *gin.Context) {
	httpapi.Respond(c, http.StatusOK, CurrentUser(c))
}

// ListUsers handles GET /api/v1/users.
func (h *Handler) ListUsers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	users, err := h.svc.ListUsers(ctx)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	items := make([]any, len(users))
	for i := range users {
		items[i] = users[i]
	}
	httpapi.RespondList(c, items, 1, len(items), len(items))
}

type userRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password,omitempty"`
	IsActive *bool  `json:"isActive,omitempty"`
}

const minPasswordLen = 12

// CreateUser handles POST /api/v1/users.
func (h *Handler) CreateUser(c *gin.Context) {
	var req userRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	fields := map[string]any{}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if !validEmail(req.Email) {
		fields["email"] = "A valid email is required."
	}
	role, ok := ParseRole(req.Role)
	if !ok {
		fields["role"] = "Unknown role."
	}
	if len(req.Password) < minPasswordLen {
		fields["password"] = "Password must be at least 12 characters."
	}
	if len(fields) > 0 {
		httpapi.FailFields(c, "Check the highlighted fields and try again.", fields)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if !CurrentUser(c).Role.Allowed(RoleSuperAdmin) {
		httpapi.Fail(c, http.StatusForbidden, httpapi.ErrForbidden, "Only the super administrator can manage users.")
		return
	}
	user, err := h.svc.CreateUser(ctx, req.Email, req.Name, role, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "23505") {
			httpapi.Fail(c, http.StatusConflict, httpapi.ErrConflict, "A user with this email already exists.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	httpapi.Respond(c, http.StatusCreated, user)
}

// UpdateUser handles PATCH /api/v1/users/:id.
func (h *Handler) UpdateUser(c *gin.Context) {
	var req userRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	role, ok := ParseRole(req.Role)
	if !ok {
		httpapi.FailFields(c, "Unknown role.", map[string]any{"role": "Unknown role."})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if len(req.Password) > 0 && len(req.Password) < 12 {
		httpapi.FailFields(c, "Use at least 12 characters.", nil)
		return
	}
	if CurrentUser(c).ID == c.Param("id") && role != RoleSuperAdmin {
		httpapi.Fail(c, 409, httpapi.ErrConflict, "You cannot demote your own account.")
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	user, err := h.svc.UpdateUser(ctx, c.Param("id"), req.Name, role, active, req.Password)
	if err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "User not found.")
		return
	}
	if req.Password != "" {
		_, err = h.pool.Exec(ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1`, user.ID)
		if err != nil {
			httpapi.InternalError(c)
			return
		}
		AuthEvent(ctx, h.pool, CurrentUser(c), "auth.admin_password_reset", c.ClientIP())
	}
	httpapi.Respond(c, http.StatusOK, user)
}

// DeleteUser handles DELETE /api/v1/users/:id.
func (h *Handler) DeleteUser(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if CurrentUser(c).ID == c.Param("id") {
		httpapi.Fail(c, 409, httpapi.ErrConflict, "You cannot delete your own account.")
		return
	}
	if err := h.svc.DeleteUser(ctx, c.Param("id")); err != nil {
		httpapi.InternalError(c)
		return
	}
	httpapi.Respond(c, http.StatusOK, gin.H{"ok": true})
}

func validEmail(s string) bool {
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		return false
	}
	domain := s[at+1:]
	if strings.Contains(domain, "..") || strings.Contains(domain, "@") {
		return false
	}
	return len(s) <= 254
}

// Register attaches the auth routes to the router group. The admin group is
// guarded by authentication, and user administration additionally requires
// the super administrator role.
func Register(r *gin.RouterGroup, pool *pgxpool.Pool, requireAuth gin.HandlerFunc, key string, secure bool) {
	h := NewHandler(pool)
	h.key = key
	h.svc.key = key
	h.secure = secure
	r.GET("/auth/status", h.HasUsers)
	r.POST("/auth/setup", PersistentRateLimit(pool), h.Setup)
	r.POST("/auth/login", PersistentRateLimit(pool), h.Login)
	r.POST("/auth/logout", requireAuth, h.Logout)
	r.GET("/auth/me", requireAuth, h.Me)

	r.POST("/auth/totp/enroll", requireAuth, h.Enroll)
	r.POST("/auth/totp/verify", requireAuth, h.VerifyEnrollment)
	r.POST("/auth/totp/disable", requireAuth, h.DisableMFA)
	r.POST("/auth/password", requireAuth, h.Password)
	r.GET("/auth/sessions", requireAuth, h.Sessions)
	r.DELETE("/auth/sessions", requireAuth, h.RevokeSessions)
	r.POST("/auth/rotate", requireAuth, h.Rotate)
	admin := r.Group("/users", requireAuth, RequireRole(RoleSuperAdmin))
	admin.GET("", h.ListUsers)
	admin.POST("", h.CreateUser)
	admin.PATCH("/:id", h.UpdateUser)
	admin.DELETE("/:id", h.DeleteUser)
}

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
	svc  *Service
	pool *pgxpool.Pool
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
	httpapi.Respond(c, http.StatusCreated, user)
}

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	user, token, err := h.svc.Login(ctx, req.Email, req.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
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
	c.SetCookie(SessionCookie, token, int(SessionLifetime.Seconds()), "/", "", false, true)
	httpapi.Respond(c, http.StatusOK, user)
}

// Logout handles POST /api/v1/auth/logout.
func (h *Handler) Logout(c *gin.Context) {
	if token, err := c.Cookie(SessionCookie); err == nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		_ = h.svc.Logout(ctx, token)
	}
	c.SetCookie(SessionCookie, "", -1, "/", "", false, true)
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
	httpapi.Respond(c, http.StatusOK, users)
}

type userRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password,omitempty"`
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
	user, err := h.svc.UpdateUser(ctx, c.Param("id"), req.Name, role, true, req.Password)
	if err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "User not found.")
		return
	}
	httpapi.Respond(c, http.StatusOK, user)
}

// DeleteUser handles DELETE /api/v1/users/:id.
func (h *Handler) DeleteUser(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
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
func Register(r *gin.RouterGroup, pool *pgxpool.Pool, requireAuth gin.HandlerFunc) {
	h := NewHandler(pool)
	r.GET("/auth/status", h.HasUsers)
	r.POST("/auth/setup", h.Setup)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/logout", h.Logout)
	r.GET("/auth/me", requireAuth, h.Me)

	admin := r.Group("/users", requireAuth)
	admin.GET("", h.ListUsers)
	admin.POST("", h.CreateUser)
	admin.PATCH("/:id", h.UpdateUser)
	admin.DELETE("/:id", h.DeleteUser)
}

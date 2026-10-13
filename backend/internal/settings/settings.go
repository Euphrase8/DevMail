// Package settings persists workspace configuration as JSON values and
// exposes it over the API.
package settings

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service reads and writes settings rows.
type Service struct {
	pool *pgxpool.Pool
}

// NewService returns a settings service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// All returns the full settings map.
func (s *Service) All(ctx context.Context) (map[string]any, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]any{}
	for rows.Next() {
		var key string
		var raw []byte
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			out[key] = string(raw)
			continue
		}
		out[key] = v
	}
	return out, rows.Err()
}

// Set stores a single settings value, merging into the returned snapshot.
func (s *Service) Set(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		key, raw)
	return err
}

// Handler exposes settings over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds the settings HTTP handler.
func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{svc: NewService(pool)}
}

// RegisterRoutes attaches the settings API to the router group behind auth.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	r.GET("/settings", auth, h.Get)
	r.PATCH("/settings", auth, h.Update)
}

// Get handles GET /api/v1/settings.
func (h *Handler) Get(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	all, err := h.svc.All(ctx)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	httpapi.Respond(c, 200, all)
}

// Update handles PATCH /api/v1/settings.
func (h *Handler) Update(c *gin.Context) {
	var req map[string]any
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, 400, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	for key, value := range req {
		if err := h.svc.Set(ctx, key, value); err != nil {
			httpapi.InternalError(c)
			return
		}
	}
	all, err := h.svc.All(ctx)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	httpapi.Respond(c, 200, all)
}

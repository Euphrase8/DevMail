// Package monitoring aggregates live operational state for the dashboard:
// infrastructure summary, per-entity health breakdowns, and recent activity.
package monitoring

import (
	"context"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service aggregates metrics from operational tables.
type Service struct {
	pool *pgxpool.Pool
}

// NewService returns the monitoring service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Summary is the dashboard payload.
type Summary struct {
	Servers   ServerSummary `json:"servers"`
	Domains   DomainSummary `json:"domains"`
	Mailboxes int           `json:"mailboxes"`
	Aliases   int           `json:"aliases"`
	Users     int           `json:"users"`
	Recent    []RecentEvent `json:"recent"`
}

// ServerSummary is the server health breakdown.
type ServerSummary struct {
	Total       int `json:"total"`
	Healthy     int `json:"healthy"`
	Degraded    int `json:"degraded"`
	Offline     int `json:"offline"`
	Pending     int `json:"pending"`
	Maintenance int `json:"maintenance"`
}

// DomainSummary is the domain verification breakdown.
type DomainSummary struct {
	Total     int `json:"total"`
	Verified  int `json:"verified"`
	Active    int `json:"active"`
	Degraded  int `json:"degraded"`
	Pending   int `json:"pending"`
	Suspended int `json:"suspended"`
}

// RecentEvent is a recent audit entry for the activity feed.
type RecentEvent struct {
	ID        int64  `json:"id"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	CreatedAt string `json:"createdAt"`
}

func count(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) (int, error) {
	var n int
	err := pool.QueryRow(ctx, query, args...).Scan(&n)
	return n, err
}

// Burndown fetches the full dashboard summary.
func (s *Service) Burndown(ctx context.Context) (*Summary, error) {
	sm := &Summary{}
	var err error

	if sm.Servers.Total, err = count(ctx, s.pool, `SELECT COUNT(*) FROM servers WHERE deleted_at IS NULL`); err != nil {
		return nil, err
	}
	for _, m := range []struct {
		col string
		dst *int
	}{
		{`status = 'healthy'`, &sm.Servers.Healthy},
		{`status = 'degraded'`, &sm.Servers.Degraded},
		{`status = 'offline'`, &sm.Servers.Offline},
		{`status = 'pending'`, &sm.Servers.Pending},
		{`maintenance = true`, &sm.Servers.Maintenance},
	} {
		q := `SELECT COUNT(*) FROM servers WHERE deleted_at IS NULL AND ` + m.col
		if *m.dst, err = count(ctx, s.pool, q); err != nil {
			return nil, err
		}
	}

	if sm.Domains.Total, err = count(ctx, s.pool, `SELECT COUNT(*) FROM domains WHERE deleted_at IS NULL`); err != nil {
		return nil, err
	}
	for _, m := range []struct {
		col string
		dst *int
	}{
		{`status = 'verified'`, &sm.Domains.Verified},
		{`status = 'active'`, &sm.Domains.Active},
		{`status = 'degraded'`, &sm.Domains.Degraded},
		{`status = 'pending'`, &sm.Domains.Pending},
		{`status = 'suspended'`, &sm.Domains.Suspended},
	} {
		q := `SELECT COUNT(*) FROM domains WHERE deleted_at IS NULL AND ` + m.col
		if *m.dst, err = count(ctx, s.pool, q); err != nil {
			return nil, err
		}
	}

	if sm.Mailboxes, err = count(ctx, s.pool, `SELECT COUNT(*) FROM mailboxes WHERE deleted_at IS NULL`); err != nil {
		return nil, err
	}
	if sm.Aliases, err = count(ctx, s.pool, `SELECT COUNT(*) FROM aliases WHERE deleted_at IS NULL`); err != nil {
		return nil, err
	}
	if sm.Users, err = count(ctx, s.pool, `SELECT COUNT(*) FROM users WHERE is_active = true`); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, actor_email, action, resource_type || ' ' || COALESCE(resource_id,''),
		       to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM audit_log ORDER BY created_at DESC, id DESC LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e RecentEvent
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Resource, &e.CreatedAt); err != nil {
			return nil, err
		}
		sm.Recent = append(sm.Recent, e)
	}
	return sm, rows.Err()
}

// Handler exposes monitoring over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds the monitoring handler.
func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{svc: NewService(pool)}
}

// RegisterRoutes attaches the monitoring API to the router group behind auth.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	r.GET("/dashboard", auth, h.Summary)
}

// Summary handles GET /api/v1/dashboard.
func (h *Handler) Summary(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	sm, err := h.svc.Burndown(ctx)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	httpapi.Respond(c, 200, sm)
}

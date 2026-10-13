// Package audit records every sensitive administrative action and exposes a
// queryable log for operators.
package audit

import (
	"context"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/auth"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Recorder writes audit entries. A nil user indicates an anonymous action.
type Recorder struct {
	pool *pgxpool.Pool
}

// NewRecorder returns an audit recorder for the given pool.
func NewRecorder(pool *pgxpool.Pool) *Recorder {
	return &Recorder{pool: pool}
}

// Record inserts an audit log entry. Sensitive fields are never written here;
// callers are responsible for redacting payloads.
func (r *Recorder) Record(ctx context.Context, user *auth.User, action, resourceType, resourceID string, detail any, ip string) {
	if r == nil || r.pool == nil {
		return
	}
	_, _ = r.pool.Exec(ctx, `
		INSERT INTO audit_log (user_id, actor_email, action, resource_type, resource_id, detail, ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		nullableUserID(user), actorEmail(user), action, resourceType, resourceID, detail, ip)
}

func nullableUserID(u *auth.User) any {
	if u == nil {
		return nil
	}
	return u.ID
}

func actorEmail(u *auth.User) string {
	if u == nil {
		return ""
	}
	return u.Email
}

// Entry is an audit log row returned by the list endpoint.
type Entry struct {
	ID           int64  `json:"id"`
	ActorEmail   string `json:"actorEmail"`
	Action       string `json:"action"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	Detail       any    `json:"detail"`
	IP           string `json:"ip"`
	CreatedAt    string `json:"createdAt"`
}

// List returns audit entries, newest first, with pagination.
func (r *Recorder) List(ctx context.Context, action string, limit, offset int) ([]Entry, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, actor_email, action, resource_type, resource_id, detail, ip,
		       to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM audit_log
		WHERE ($1 = '' OR action = $1)
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`, action, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.ActorEmail, &e.Action, &e.ResourceType, &e.ResourceID, &e.Detail, &e.IP, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// Handler exposes the audit log over HTTP.
type Handler struct {
	rec *Recorder
}

// NewHandler builds the audit HTTP handler.
func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{rec: NewRecorder(pool)}
}

// RegisterRoutes attaches the audit API to the router group behind auth.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	r.GET("/audit", auth, h.List)
}

// List handles GET /api/v1/audit?action=&page=&perPage=.
func (h *Handler) List(c *gin.Context) {
	limit := 50
	if v := c.Query("perPage"); v != "" {
		if n := atoi(v); n > 0 && n <= 200 {
			limit = n
		}
	}
	page := atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	entries, total, err := h.rec.List(ctx, c.Query("action"), limit, (page-1)*limit)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	data := make([]any, len(entries))
	for i := range entries {
		data[i] = entries[i]
	}
	httpapi.RespondList(c, data, page, limit, total)
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

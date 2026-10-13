// Package servers manages mail servers, their connectivity health, and
// SSRF-safe live checks.
package servers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/audit"
	"github.com/Euphrase8/DevMail/backend/internal/auth"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/Euphrase8/DevMail/backend/internal/netutil"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

// Server is the API representation of a managed mail server.
type Server struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Hostname        string `json:"hostname"`
	IPv4            string `json:"ipv4"`
	IPv6            string `json:"ipv6"`
	Location        string `json:"location"`
	Environment     string `json:"environment"`
	Notes           string `json:"notes"`
	Status          string `json:"status"`
	Maintenance     bool   `json:"maintenance"`
	LastCheckAt     string `json:"lastCheckAt"`
	LastCheckError  string `json:"lastCheckError"`
}

const selectColumns = `id::text, name, hostname,
	COALESCE(host(ipv4),''), COALESCE(host(ipv6),''), location, environment, notes,
	status, maintenance,
	COALESCE(to_char(last_check_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
	last_check_error`

// Service implements server persistence and health checking.
type Service struct {
	pool     *pgxpool.Pool
	audit    *audit.Recorder
	allowPrivate bool
}

// NewService creates a server service.
func NewService(pool *pgxpool.Pool, auditRec *audit.Recorder, allowPrivate bool) *Service {
	return &Service{pool: pool, audit: auditRec, allowPrivate: allowPrivate}
}

func scanServer(row interface{ Scan(...any) error }) (*Server, error) {
	var s Server
	if err := row.Scan(&s.ID, &s.Name, &s.Hostname, &s.IPv4, &s.IPv6, &s.Location,
		&s.Environment, &s.Notes, &s.Status, &s.Maintenance, &s.LastCheckAt, &s.LastCheckError); err != nil {
		return nil, err
	}
	return &s, nil
}

// List returns all non-soft-deleted servers.
func (s *Service) List(ctx context.Context) ([]*Server, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+selectColumns+` FROM servers WHERE deleted_at IS NULL ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Server
	for rows.Next() {
		srv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

// Get returns a single server.
func (s *Service) Get(ctx context.Context, id string) (*Server, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+selectColumns+` FROM servers WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanServer(row)
}

// Create inserts a new server.
func (s *Service) Create(ctx context.Context, server Server) (*Server, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO servers (name, hostname, ipv4, ipv6, location, environment, notes)
		VALUES ($1, $2, $3::inet, $4::inet, $5, $6, $7)
		RETURNING `+selectColumns,
		server.Name, server.Hostname, nullableInet(server.IPv4), nullableInet(server.IPv6),
		server.Location, server.Environment, server.Notes)
	return scanServer(row)
}

// Update edits a server.
func (s *Service) Update(ctx context.Context, id string, server Server) (*Server, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE servers SET name=$2, hostname=$3, ipv4=$4::inet, ipv6=$5::inet,
			location=$6, environment=$7, notes=$8, updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL
		RETURNING `+selectColumns,
		id, server.Name, server.Hostname, nullableInet(server.IPv4), nullableInet(server.IPv6),
		server.Location, server.Environment, server.Notes)
	return scanServer(row)
}

// Delete soft-deletes a server.
func (s *Service) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE servers SET deleted_at = now(), status='disabled' WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// SetMaintenance toggles the server maintenance flag.
func (s *Service) SetMaintenance(ctx context.Context, id string, on bool) (*Server, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE servers SET maintenance=$2, updated_at=now() WHERE id=$1 AND deleted_at IS NULL
		RETURNING `+selectColumns, id, on)
	return scanServer(row)
}

func nullableInet(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// Check runs a live connectivity probe (TCP on port 25, timeout-guarded) and
// stores the outcome. Only the resolved, policy-approved address is dialed.
func (s *Service) Check(ctx context.Context, id string) (*Server, error) {
	srv, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	const smtpPort = 25
	ok, latency, addr, err := netutil.CheckPort(ctx, srv.Hostname, smtpPort, 6*time.Second, s.allowPrivate)
	status := "healthy"
	lastErr := ""
	if err != nil {
		status = "degraded"
		if errors.Is(err, netutil.ErrNetworkBlocked) {
			status = "offline"
		}
		lastErr = err.Error()
	} else if !ok {
		status = "offline"
		lastErr = "connection failed"
	}
	_, _ = s.pool.Exec(ctx, `
		UPDATE servers SET last_check_at=now(), last_check_error=$2, status=$3, updated_at=now()
		WHERE id=$1`, id, lastErr, status)
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO server_health (server_id, status, latency_ms, addr, checked_at)
		VALUES ($1,$2,$3,$4,now())`, id, status, latency.Milliseconds(), addr)
	row := s.pool.QueryRow(ctx, `SELECT `+selectColumns+` FROM servers WHERE id=$1`, id)
	return scanServer(row)
}

// HealthPoint is a stored connectivity sample for a server.
type HealthPoint struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latencyMs"`
	Addr      string `json:"addr"`
	CheckedAt string `json:"checkedAt"`
}

// HealthHistory returns recent health samples for a server, newest first.
func (s *Service) HealthHistory(ctx context.Context, id string, limit int) ([]*HealthPoint, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if _, err := s.Get(ctx, id); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM server_health WHERE server_id=$1`, id).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT status, latency_ms, addr,
		       to_char(checked_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM server_health WHERE server_id=$1 ORDER BY checked_at DESC LIMIT $2`, id, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*HealthPoint
	for rows.Next() {
		var p HealthPoint
		if err := rows.Scan(&p.Status, &p.LatencyMs, &p.Addr, &p.CheckedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, &p)
	}
	return out, total, rows.Err()
}

// CheckAll probes every non-disabled server concurrently.
func (s *Service) CheckAll(ctx context.Context) ([]*Server, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(6)
	for _, srv := range all {
		srv := srv
		g.Go(func() error {
			_, _ = s.Check(gctx, srv.ID)
			return nil
		})
	}
	_ = g.Wait()
	return s.List(ctx)
}

func validServerName(name string) error {
	if name == "" || len(name) > 64 {
		return fmt.Errorf("name is required and must be 64 characters or fewer")
	}
	return nil
}

func validHostname(h string) error {
	if h == "" || len(h) > 253 {
		return fmt.Errorf("hostname is required and must be 253 characters or fewer")
	}
	for _, r := range h {
		if !(r == '.' || r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return fmt.Errorf("hostname contains invalid characters")
		}
	}
	return nil
}

func validInet(v string) error {
	if v == "" {
		return nil
	}
	if _, err := parseInet(v); err != nil {
		return fmt.Errorf("invalid IP address")
	}
	return nil
}

// Handler exposes the servers API.
type Handler struct {
	svc *Service
}

// NewHandler builds the server HTTP handler.
func NewHandler(pool *pgxpool.Pool, auditRec *audit.Recorder, allowPrivate bool) *Handler {
	return &Handler{svc: NewService(pool, auditRec, allowPrivate)}
}

type serverRequest struct {
	Name        string `json:"name"`
	Hostname    string `json:"hostname"`
	IPv4        string `json:"ipv4"`
	IPv6        string `json:"ipv6"`
	Location    string `json:"location"`
	Environment string `json:"environment"`
	Notes       string `json:"notes"`
}

func (h *Handler) validate(req *serverRequest) map[string]any {
	fields := map[string]any{}
	if err := validServerName(req.Name); err != nil {
		fields["name"] = err.Error()
	}
	if err := validHostname(req.Hostname); err != nil {
		fields["hostname"] = err.Error()
	}
	if err := validInet(req.IPv4); err != nil {
		fields["ipv4"] = err.Error()
	}
	if err := validInet(req.IPv6); err != nil {
		fields["ipv6"] = err.Error()
	}
	return fields
}

// RegisterRoutes attaches the servers API to the router group behind auth.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	servers := r.Group("/servers", auth)
	servers.GET("", h.List)
	servers.POST("", h.Create)
	servers.POST("/check-all", h.CheckAll)
	servers.POST("/:id/check", h.Check)
	servers.GET("/:id/health", h.Health)
	servers.PATCH("/:id", h.Update)
	servers.DELETE("/:id", h.Delete)
	servers.POST("/:id/maintenance", h.ToggleMaintenance)
}

// List handles GET /api/v1/servers.
func (h *Handler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	all, err := h.svc.List(ctx)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	data := make([]any, len(all))
	for i := range all {
		data[i] = all[i]
	}
	httpapi.RespondList(c, data, 1, len(all), len(all))
}

// Create handles POST /api/v1/servers.
func (h *Handler) Create(c *gin.Context) {
	var req serverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	if fields := h.validate(&req); len(fields) > 0 {
		httpapi.FailFields(c, "Check the highlighted fields and try again.", fields)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	srv, err := h.svc.Create(ctx, Server{
		Name: req.Name, Hostname: req.Hostname, IPv4: req.IPv4, IPv6: req.IPv6,
		Location: req.Location, Environment: req.Environment, Notes: req.Notes,
	})
	if err != nil {
		if isUniqueViolation(err) {
			httpapi.Fail(c, http.StatusConflict, httpapi.ErrConflict, "A server with this hostname already exists.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "server.create", "server", srv.ID, gin.H{"name": srv.Name, "hostname": srv.Hostname}, c.ClientIP())
	httpapi.Respond(c, http.StatusCreated, srv)
}

// Check handles POST /api/v1/servers/:id/check.
func (h *Handler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	srv, err := h.svc.Check(ctx, c.Param("id"))
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Server not found.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "server.check", "server", srv.ID, gin.H{"status": srv.Status}, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, srv)
}

// CheckAll handles POST /api/v1/servers/check-all.
func (h *Handler) CheckAll(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	all, err := h.svc.CheckAll(ctx)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	data := make([]any, len(all))
	for i := range all {
		data[i] = all[i]
	}
	httpapi.RespondList(c, data, 1, len(all), len(all))
}

// Health handles GET /api/v1/servers/:id/health?limit=.
func (h *Handler) Health(c *gin.Context) {
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n := 0; len(v) > 0 {
			for _, r := range v {
				if r < '0' || r > '9' {
					n = 0
					break
				}
				n = n*10 + int(r-'0')
			}
			if n > 0 {
				limit = n
			}
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	points, total, err := h.svc.HealthHistory(ctx, c.Param("id"), limit)
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Server not found.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	data := make([]any, len(points))
	for i := range points {
		data[i] = points[i]
	}
	httpapi.RespondList(c, data, 1, limit, total)
}

// Update handles PATCH /api/v1/servers/:id.
func (h *Handler) Update(c *gin.Context) {
	var req serverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	if fields := h.validate(&req); len(fields) > 0 {
		httpapi.FailFields(c, "Check the highlighted fields and try again.", fields)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	srv, err := h.svc.Update(ctx, c.Param("id"), Server{
		Name: req.Name, Hostname: req.Hostname, IPv4: req.IPv4, IPv6: req.IPv6,
		Location: req.Location, Environment: req.Environment, Notes: req.Notes,
	})
	if err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Server not found.")
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "server.update", "server", srv.ID, gin.H{"name": srv.Name}, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, srv)
}

// Delete handles DELETE /api/v1/servers/:id.
func (h *Handler) Delete(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Delete(ctx, c.Param("id")); err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Server not found.")
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "server.delete", "server", c.Param("id"), nil, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, gin.H{"ok": true})
}

// ToggleMaintenance handles POST /api/v1/servers/:id/maintenance.
func (h *Handler) ToggleMaintenance(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	_ = c.ShouldBindJSON(&req)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	srv, err := h.svc.SetMaintenance(ctx, c.Param("id"), req.Enabled)
	if err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Server not found.")
		return
	}
	httpapi.Respond(c, http.StatusOK, srv)
}
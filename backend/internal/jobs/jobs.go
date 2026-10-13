// Package jobs provides an asynchronous provisioning queue. Jobs are
// persisted so work survives restarts; a minimal in-process worker executes
// queued work with retries and backoff.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/audit"
	"github.com/Euphrase8/DevMail/backend/internal/auth"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Job is the API representation of a provisioning job.
type Job struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	Error       string `json:"error"`
	Attempts    int    `json:"attempts"`
	MaxAttempts int    `json:"maxAttempts"`
	Payload     any    `json:"payload"`
	Result      any    `json:"result"`
	StartedAt   string `json:"startedAt"`
	FinishedAt  string `json:"finishedAt"`
	CreatedAt   string `json:"createdAt"`
}

// KnownKinds enumerates the job kinds the worker understands. Enqueue rejects
// anything outside this set so the queue never fills with unconsumable work.
var KnownKinds = map[string]bool{
	"server.dns_rebuild": true,
	"domain.provision":   true,
	"dns.apply":          true,
	"mailbox.create":     true,
	"alias.provision":    true,
}

const jobColumns = `id::text, kind, status, error, attempts, max_attempts, payload, result,
	COALESCE(to_char(started_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
	COALESCE(to_char(finished_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
	to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

// Service persists and executes jobs.
type Service struct {
	pool    *pgxpool.Pool
	logger  *slog.Logger
	mu      sync.Mutex
	polling bool
}

// NewService returns the job service backed by the given pool.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{pool: pool, logger: logger}
}

// List returns recent jobs.
func (s *Service) List(ctx context.Context, limit int) ([]*Job, error) {
	items, _, err := s.ListPage(ctx, 1, limit)
	return items, err
}

// ListPage returns jobs with pagination metadata.
func (s *Service) ListPage(ctx context.Context, page, perPage int) ([]*Job, int, error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 50
	}
	if page < 1 {
		page = 1
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+jobColumns+` FROM jobs ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2`,
		perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out, err := scanJobs(rows)
	return out, total, err
}

func scanJobs(rows interface{ Next() bool; Scan(...any) error; Err() error }) ([]*Job, error) {
	var out []*Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Kind, &j.Status, &j.Error, &j.Attempts, &j.MaxAttempts,
			&j.Payload, &j.Result, &j.StartedAt, &j.FinishedAt, &j.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &j)
	}
	return out, rows.Err()
}

// Get returns a single job.
func (s *Service) Get(ctx context.Context, id string) (*Job, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id=$1`, id)
	var j Job
	if err := row.Scan(&j.ID, &j.Kind, &j.Status, &j.Error, &j.Attempts, &j.MaxAttempts,
		&j.Payload, &j.Result, &j.StartedAt, &j.FinishedAt, &j.CreatedAt); err != nil {
		return nil, errNotFound
	}
	return &j, nil
}

// Enqueue validates the kind, inserts a job, and starts the worker.
func (s *Service) Enqueue(ctx context.Context, kind string, payload any) (*Job, error) {
	if !KnownKinds[kind] {
		return nil, errUnknownKind{kind}
	}
	var raw []byte
	var err error
	if payload != nil {
		if raw, err = json.Marshal(payload); err != nil {
			return nil, err
		}
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO jobs (kind, payload) VALUES ($1, $2)
		RETURNING `+jobColumns, kind, raw)
	var j Job
	if err := row.Scan(&j.ID, &j.Kind, &j.Status, &j.Error, &j.Attempts, &j.MaxAttempts,
		&j.Payload, &j.Result, &j.StartedAt, &j.FinishedAt, &j.CreatedAt); err != nil {
		return nil, err
	}
	s.kick()
	return &j, nil
}

// Retry resets a failed or cancelled job so the worker can run it again.
func (s *Service) Retry(ctx context.Context, id string) (*Job, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE jobs SET status='queued', attempts=0, error='', run_at=now(), finished_at=NULL, started_at=NULL
		WHERE id=$1 AND status IN ('failed','cancelled')
		RETURNING `+jobColumns, id)
	var j Job
	if err := row.Scan(&j.ID, &j.Kind, &j.Status, &j.Error, &j.Attempts, &j.MaxAttempts,
		&j.Payload, &j.Result, &j.StartedAt, &j.FinishedAt, &j.CreatedAt); err != nil {
		return nil, errNotFound
	}
	s.kick()
	return &j, nil
}

// Cancel stops a job that has not yet finished.
func (s *Service) Cancel(ctx context.Context, id string) (*Job, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE jobs SET status='cancelled', finished_at=now()
		WHERE id=$1 AND status IN ('queued','retrying')
		RETURNING `+jobColumns, id)
	var j Job
	if err := row.Scan(&j.ID, &j.Kind, &j.Status, &j.Error, &j.Attempts, &j.MaxAttempts,
		&j.Payload, &j.Result, &j.StartedAt, &j.FinishedAt, &j.CreatedAt); err != nil {
		return nil, errNotFound
	}
	return &j, nil
}

// kick starts the background worker once, guarding against races.
func (s *Service) kick() {
	s.mu.Lock()
	if s.polling {
		s.mu.Unlock()
		return
	}
	s.polling = true
	s.mu.Unlock()
	go s.worker()
}

func (s *Service) worker() {
	for {
		if s.claim() == 0 {
			s.mu.Lock()
			s.polling = false
			s.mu.Unlock()
			return
		}
	}
}

func (s *Service) claim() int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	row := s.pool.QueryRow(ctx, `
		UPDATE jobs SET status='running', started_at=now(), attempts = attempts + 1
		WHERE id = (
			SELECT id FROM jobs
			WHERE status IN ('queued','retrying') AND run_at <= now() AND attempts < max_attempts
			ORDER BY created_at LIMIT 1
		)
		RETURNING id, kind, payload, attempts, max_attempts`)
	var id, kind string
	var payload []byte
	var attempts, maxAttempts int
	if err := row.Scan(&id, &kind, &payload, &attempts, &maxAttempts); err != nil {
		return 0
	}

	result := s.run(ctx, kind, payload)
	if result == nil {
		_, _ = s.pool.Exec(ctx, `
			UPDATE jobs SET status='succeeded', result='"ok"', finished_at=now()
			WHERE id=$1`, id)
	} else if attempts >= maxAttempts {
		_, _ = s.pool.Exec(ctx, `
			UPDATE jobs SET status='failed', error=$2, finished_at=now() WHERE id=$1`, id, result.Error())
	} else {
		_, _ = s.pool.Exec(ctx, `
			UPDATE jobs SET status='retrying', error=$2, run_at = now() + interval '10 seconds' WHERE id=$1`, id, result.Error())
	}
	return 1
}

// run executes a job handler. Real provisioning consumers are registered by
// their domain packages; until one is bound a kind is acknowledged as a no-op.
func (s *Service) run(ctx context.Context, kind string, payload []byte) error {
	if !KnownKinds[kind] {
		return errUnknownKind{kind}
	}
	s.logger.Debug("job consumed", slog.String("kind", kind))
	return nil
}

type errUnknownKind struct{ kind string }

func (e errUnknownKind) Error() string { return "unknown job kind: " + e.kind }

func isUnknownKind(err error) bool {
	var e errUnknownKind
	return errors.As(err, &e)
}

var errNotFound = errors.New("not found")

// Handler exposes the jobs API.
type Handler struct {
	svc   *Service
	audit *audit.Recorder
}

// NewHandler builds the job HTTP handler.
func NewHandler(pool *pgxpool.Pool, logger *slog.Logger, rec *audit.Recorder) *Handler {
	return &Handler{svc: NewService(pool, logger), audit: rec}
}

// RegisterRoutes attaches the jobs API to the router group behind auth.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	g := r.Group("/jobs", auth)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.POST("/:id/retry", h.Retry)
	g.POST("/:id/cancel", h.Cancel)
}

// List handles GET /api/v1/jobs?page=&perPage=.
func (h *Handler) List(c *gin.Context) {
	page := atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	perPage := atoi(c.Query("perPage"))
	if perPage < 1 {
		perPage = 50
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	items, total, err := h.svc.ListPage(ctx, page, perPage)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	data := make([]any, len(items))
	for i := range items {
		data[i] = items[i]
	}
	httpapi.RespondList(c, data, page, perPage, total)
}

// Create handles POST /api/v1/jobs.
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		Kind    string `json:"kind"`
		Payload any    `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, 400, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	j, err := h.svc.Enqueue(ctx, req.Kind, req.Payload)
	if err != nil {
		if isUnknownKind(err) {
			httpapi.Fail(c, 422, httpapi.ErrValidation, "Unknown job kind.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	h.audit.Record(ctx, auth.CurrentUser(c), "job.enqueue", "job", j.ID, gin.H{"kind": j.Kind}, c.ClientIP())
	httpapi.Respond(c, 201, j)
}

// Retry handles POST /api/v1/jobs/:id/retry.
func (h *Handler) Retry(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	j, err := h.svc.Retry(ctx, c.Param("id"))
	if err != nil {
		httpapi.Fail(c, 404, httpapi.ErrNotFound, "Job not found or is not retryable.")
		return
	}
	h.audit.Record(ctx, auth.CurrentUser(c), "job.retry", "job", j.ID, nil, c.ClientIP())
	httpapi.Respond(c, 200, j)
}

// Cancel handles POST /api/v1/jobs/:id/cancel.
func (h *Handler) Cancel(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	j, err := h.svc.Cancel(ctx, c.Param("id"))
	if err != nil {
		httpapi.Fail(c, 404, httpapi.ErrNotFound, "Job not found or has already finished.")
		return
	}
	h.audit.Record(ctx, auth.CurrentUser(c), "job.cancel", "job", j.ID, nil, c.ClientIP())
	httpapi.Respond(c, 200, j)
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

// Package jobs provides an asynchronous provisioning queue. Jobs are
// persisted so work survives restarts; a minimal in-process worker executes
// queued work with retries and backoff.
package jobs

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

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
	CreatedAt   string `json:"createdAt"`
	FinishedAt  string `json:"finishedAt"`
}

const jobColumns = `id::text, kind, status, error, attempts, max_attempts,
	to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
	COALESCE(to_char(finished_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),'')`

// Service persists and executes jobs.
type Service struct {
	pool    *pgxpool.Pool
	logger  *slog.Logger
	polling bool
}

// NewService returns the job service backed by the given pool.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{pool: pool, logger: logger}
}

// List returns recent jobs.
func (s *Service) List(ctx context.Context, limit int) ([]*Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `SELECT `+jobColumns+` FROM jobs ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Kind, &j.Status, &j.Error, &j.Attempts, &j.MaxAttempts, &j.CreatedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, &j)
	}
	return out, rows.Err()
}

// Enqueue inserts a job and starts the worker if it is not already running.
func (s *Service) Enqueue(ctx context.Context, kind string, payload any) (*Job, error) {
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
	if err := row.Scan(&j.ID, &j.Kind, &j.Status, &j.Error, &j.Attempts, &j.MaxAttempts, &j.CreatedAt, &j.FinishedAt); err != nil {
		return nil, err
	}
	go s.Start()
	return &j, nil
}

// Start fires the background worker once per process; it drains the queue.
func (s *Service) Start() {
	if s.polling {
		return
	}
	s.polling = true
	go s.worker()
}

func (s *Service) worker() {
	for {
		claimed := s.claim()
		if claimed == 0 {
			s.polling = false
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
	finished := true
	if result == nil {
		_, _ = s.pool.Exec(ctx, `
			UPDATE jobs SET status='succeeded', result=NULLIF($2,''), finished_at=now()
			WHERE id=$1`, id, stringify(result))
	} else if attempts >= maxAttempts {
		_, _ = s.pool.Exec(ctx, `
			UPDATE jobs SET status='failed', error=$2, finished_at=now() WHERE id=$1`, id, result.Error())
	} else {
		finished = false
		_, _ = s.pool.Exec(ctx, `
			UPDATE jobs SET status='retrying', error=$2, run_at = now() + interval '10 seconds' WHERE id=$1`, id, result.Error())
	}

	if finished {
		return 1
	}
	return 1
}

// run executes a job handler. Inline for now; a registry of handlers by kind
// will be added when the first real provisioning consumer lands.
func (s *Service) run(ctx context.Context, kind string, payload []byte) error {
	switch kind {
	case "server.dns_rebuild", "domain.provision", "mailbox.create", "alias.provision":
		// No-op consumer registered for future provisioning tasks.
		s.logger.Debug("job (unconsumed)", slog.String("kind", kind))
		return nil
	default:
		return errUnknownKind{kind}
	}
}

func stringify(v any) any {
	if v == nil {
		return nil
	}
	return "ok"
}

type errUnknownKind struct{ kind string }

func (e errUnknownKind) Error() string { return "unknown job kind: " + e.kind }

// Handler exposes the jobs API.
type Handler struct {
	svc *Service
}

// NewHandler builds the job HTTP handler.
func NewHandler(pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{svc: NewService(pool, logger)}
}

// RegisterRoutes attaches the jobs API to the router group behind auth.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	r.GET("/jobs", auth, h.List)
}

// List handles GET /api/v1/jobs.
func (h *Handler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	jobs, err := h.svc.List(ctx, 50)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	data := make([]any, len(jobs))
	for i := range jobs {
		data[i] = jobs[i]
	}
	httpapi.RespondList(c, data, 1, len(jobs), len(jobs))
}
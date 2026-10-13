package providers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/audit"
	"github.com/Euphrase8/DevMail/backend/internal/auth"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/Euphrase8/DevMail/backend/internal/secretbox"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Provider is the API representation of a DNS provider connection. The
// credential (token) is never returned — only whether one is stored.
type Provider struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	Zone            string `json:"zone"`
	Status          string `json:"status"`
	HasToken        bool   `json:"hasToken"`
	LastVerifiedAt  string `json:"lastVerifiedAt"`
	LastVerifyError string `json:"lastVerifyError"`
	CreatedAt       string `json:"createdAt"`
}

// Record is a single DNS change requested from a provider adapter.
type Record struct {
	Type     string
	Name     string
	Value    string
	TTL      int
	Priority int
}

// Result reports the outcome of applying one record.
type Result struct {
	Type   string
	Name   string
	Status string // published | manual | error
	Detail string
}

// Adapter is the pluggable boundary for a DNS provider. Implementations push
// (or describe) records in a zone without touching handlers.
type Adapter interface {
	Kind() string
	// Verify confirms the stored credentials can reach the zone.
	Verify(ctx context.Context) error
	// Apply creates or updates each record idempotently.
	Apply(ctx context.Context, records []Record) ([]Result, error)
}

const providerColumns = `id::text, name, kind, zone, status, (token_enc <> ''),
	COALESCE(to_char(last_verified_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
	last_verify_error,
	to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

type rowScanner interface{ Scan(...any) error }

func scanProvider(row rowScanner) (*Provider, error) {
	var p Provider
	if err := row.Scan(&p.ID, &p.Name, &p.Kind, &p.Zone, &p.Status, &p.HasToken,
		&p.LastVerifiedAt, &p.LastVerifyError, &p.CreatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

// Service manages provider rows and builds adapters from them.
type Service struct {
	pool       *pgxpool.Pool
	passphrase string
	httpClient *http.Client
}

// NewService returns a provider service. The passphrase encrypts credentials.
func NewService(pool *pgxpool.Pool, passphrase string) *Service {
	return &Service{
		pool:       pool,
		passphrase: passphrase,
		httpClient: &http.Client{Timeout: 12 * time.Second},
	}
}

// List returns all non-deleted providers.
func (s *Service) List(ctx context.Context) ([]*Provider, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+providerColumns+` FROM dns_providers WHERE deleted_at IS NULL ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Provider
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get returns a provider by id.
func (s *Service) Get(ctx context.Context, id string) (*Provider, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+providerColumns+` FROM dns_providers WHERE id=$1 AND deleted_at IS NULL`, id)
	return scanProvider(row)
}

// Create inserts a provider, encrypting the token when supplied.
func (s *Service) Create(ctx context.Context, name, kind, zone, token string) (*Provider, error) {
	enc, err := secretbox.Seal(s.passphrase, token)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO dns_providers (name, kind, zone, token_enc, status)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING `+providerColumns,
		name, kind, zone, enc, initialStatus(kind, token))
	return scanProvider(row)
}

// Update edits a provider. An empty token keeps the existing credential.
func (s *Service) Update(ctx context.Context, id, name, kind, zone, token string) (*Provider, error) {
	if token == "" {
		row := s.pool.QueryRow(ctx, `
			UPDATE dns_providers SET name=$2, kind=$3, zone=$4, updated_at=now()
			WHERE id=$1 AND deleted_at IS NULL RETURNING `+providerColumns,
			id, name, kind, zone)
		return scanProvider(row)
	}
	enc, err := secretbox.Seal(s.passphrase, token)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		UPDATE dns_providers SET name=$2, kind=$3, zone=$4, token_enc=$5, status=$6, updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL RETURNING `+providerColumns,
		id, name, kind, zone, enc, initialStatus(kind, token))
	return scanProvider(row)
}

// Delete soft-deletes a provider.
func (s *Service) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE dns_providers SET deleted_at=now(), status='disabled' WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

const providerTokenQuery = `SELECT kind, zone, token_enc FROM dns_providers WHERE id=$1 AND deleted_at IS NULL`

// Adapter builds the adapter for a provider id.
func (s *Service) Adapter(ctx context.Context, id string) (Adapter, *Provider, error) {
	var kind, zone, enc string
	if err := s.pool.QueryRow(ctx, providerTokenQuery, id).Scan(&kind, &zone, &enc); err != nil {
		return nil, nil, errNotFound
	}
	token, err := secretbox.Open(s.passphrase, enc)
	if err != nil {
		return nil, nil, err
	}
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	switch kind {
	case "cloudflare":
		return &cloudflareAdapter{zone: zone, token: token, client: s.httpClient}, p, nil
	default:
		return &manualAdapter{}, p, nil
	}
}

// Verify runs the adapter's credential check and records the outcome.
func (s *Service) Verify(ctx context.Context, id string) (*Provider, error) {
	adapter, p, err := s.Adapter(ctx, id)
	if err != nil {
		return nil, err
	}
	var verr error
	if p.Kind == "manual" {
		verr = nil
	} else if p.HasToken {
		verr = adapter.Verify(ctx)
	} else {
		verr = errors.New("no credential is stored for this provider")
	}
	status := "connected"
	msg := ""
	if verr != nil {
		status = "error"
		msg = verr.Error()
	}
	row := s.pool.QueryRow(ctx, `
		UPDATE dns_providers SET status=$2, last_verified_at=now(), last_verify_error=$3, updated_at=now()
		WHERE id=$1 RETURNING `+providerColumns, id, status, msg)
	return scanProvider(row)
}

// errNotFound is returned when a provider id does not resolve.
var errNotFound = errors.New("not found")

// initialStatus sets a new provider's status from its kind and credential.
func initialStatus(kind, token string) string {
	if kind == "manual" {
		return "connected"
	}
	if token == "" {
		return "pending"
	}
	return "pending"
}

// Handler exposes the providers API.
type Handler struct {
	svc   *Service
	audit *audit.Recorder
}

// NewHandler builds the providers HTTP handler.
func NewHandler(pool *pgxpool.Pool, passphrase string, rec *audit.Recorder) *Handler {
	return &Handler{svc: NewService(pool, passphrase), audit: rec}
}

// Service returns the underlying service (used by the DNS apply flow).
func (h *Handler) Service() *Service { return h.svc }

var providerKinds = map[string]bool{"manual": true, "cloudflare": true, "route53": true}

type providerRequest struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Zone  string `json:"zone"`
	Token string `json:"token"`
}

func validateProvider(req *providerRequest, creating bool) map[string]any {
	fields := map[string]any{}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 80 {
		fields["name"] = "A name of up to 80 characters is required."
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if creating {
		if !providerKinds[kind] {
			fields["kind"] = "Choose a supported provider kind."
		}
	}
	if kind != "manual" && kind != "" && strings.TrimSpace(req.Zone) == "" {
		fields["zone"] = "A zone name or id is required for API providers."
	}
	return fields
}

// RegisterRoutes attaches the providers API to the router group behind auth.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	p := r.Group("/providers", auth)
	p.GET("", h.List)
	p.POST("", h.Create)
	p.PATCH("/:id", h.Update)
	p.DELETE("/:id", h.Delete)
	p.POST("/:id/verify", h.Verify)
}

// List handles GET /api/v1/providers.
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

// Create handles POST /api/v1/providers.
func (h *Handler) Create(c *gin.Context) {
	var req providerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	req.Kind = strings.ToLower(strings.TrimSpace(req.Kind))
	if fields := validateProvider(&req, true); len(fields) > 0 {
		httpapi.FailFields(c, "Check the highlighted fields and try again.", fields)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	p, err := h.svc.Create(ctx, req.Name, req.Kind, req.Zone, req.Token)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	h.audit.Record(ctx, auth.CurrentUser(c), "provider.create", "provider", p.ID,
		gin.H{"name": p.Name, "kind": p.Kind}, c.ClientIP())
	httpapi.Respond(c, http.StatusCreated, p)
}

// Update handles PATCH /api/v1/providers/:id.
func (h *Handler) Update(c *gin.Context) {
	var req providerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	req.Kind = strings.ToLower(strings.TrimSpace(req.Kind))
	if fields := validateProvider(&req, false); len(fields) > 0 {
		httpapi.FailFields(c, "Check the highlighted fields and try again.", fields)
		return
	}
	if req.Kind == "" {
		req.Kind = "manual"
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	p, err := h.svc.Update(ctx, c.Param("id"), req.Name, req.Kind, req.Zone, req.Token)
	if err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Provider not found.")
		return
	}
	h.audit.Record(ctx, auth.CurrentUser(c), "provider.update", "provider", p.ID, gin.H{"name": p.Name}, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, p)
}

// Delete handles DELETE /api/v1/providers/:id.
func (h *Handler) Delete(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Delete(ctx, c.Param("id")); err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Provider not found.")
		return
	}
	h.audit.Record(ctx, auth.CurrentUser(c), "provider.delete", "provider", c.Param("id"), nil, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, gin.H{"ok": true})
}

// Verify handles POST /api/v1/providers/:id/verify.
func (h *Handler) Verify(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	p, err := h.svc.Verify(ctx, c.Param("id"))
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Provider not found.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	h.audit.Record(ctx, auth.CurrentUser(c), "provider.verify", "provider", p.ID, gin.H{"status": p.Status}, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, p)
}



// Package mailboxes manages mailbox and alias records on managed domains.
// The optional engine adapter (Dovecot virtual users, LMTP, an external
// provider) is a pluggable boundary: when no engine is configured the API
// still manages the database records and reports engineStatus "unmanaged".
package mailboxes

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/audit"
	"github.com/Euphrase8/DevMail/backend/internal/auth"
	"github.com/Euphrase8/DevMail/backend/internal/engine"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Mailbox is the API representation of a mailbox on a managed domain.
type Mailbox struct {
	ID            string `json:"id"`
	DomainID      string `json:"domainId"`
	Local         string `json:"local"`
	Address       string `json:"address"`
	DisplayName   string `json:"displayName"`
	QuotaBytes    int64  `json:"quotaBytes"`
	Status        string `json:"status"`
	EngineStatus  string `json:"engineStatus"`
	PasswordSetAt string `json:"passwordSetAt"`
	CreatedAt     string `json:"createdAt"`
}

// mailboxColsJoined is used when mailboxes is joined to domains (the address
// is derived from the joined row).
const mailboxColsJoined = `mb.id::text, mb.domain_id::text, mb.local, (mb.local || '@' || d.name),
	mb.display_name, mb.quota_bytes, mb.status, mb.engine_status,
	COALESCE(to_char(mb.password_set_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
	to_char(mb.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

// mailboxColsPlain is used in INSERT/UPDATE RETURNING clauses where no alias
// is available; the address is resolved via a scalar subquery.
const mailboxColsPlain = `id::text, domain_id::text, local,
	(local || '@' || (SELECT d.name FROM domains d WHERE d.id = domain_id AND d.deleted_at IS NULL)),
	display_name, quota_bytes, status, engine_status,
	COALESCE(to_char(password_set_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
	to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

// Enqueuer schedules an asynchronous provisioning job. Wiring passes a closure
// over the jobs service so this package stays free of a jobs dependency.
type Enqueuer func(ctx context.Context, kind string, payload map[string]string) error

// Service implements mailbox and alias persistence and enqueues provisioning.
type Service struct {
	pool    *pgxpool.Pool
	audit   *audit.Recorder
	engine  engine.Engine
	enqueue Enqueuer
}

// NewService creates the mailbox service with the given engine adapter.
func NewService(pool *pgxpool.Pool, rec *audit.Recorder, eng engine.Engine, enqueue Enqueuer) *Service {
	if eng == nil {
		eng = engine.Noop{}
	}
	return &Service{pool: pool, audit: rec, engine: eng, enqueue: enqueue}
}

// EngineStatus returns the current engine configuration.
func (s *Service) EngineStatus() map[string]any {
	return map[string]any{"configured": s.engine.Configured(), "name": s.engine.Name()}
}

// enqueueProvision schedules a job when a live engine is configured.
func (s *Service) enqueueProvision(ctx context.Context, kind, idName, id string) {
	if !s.engine.Configured() || s.enqueue == nil {
		return
	}
	_ = s.enqueue(ctx, kind, map[string]string{idName: id})
}

// ProvisionTarget resolves the data a mailbox provisioning consumer needs.
func (s *Service) ProvisionTarget(ctx context.Context, id string) (local, domain, passwordHash string, quotaBytes int64, err error) {
	// Soft-deleted mailboxes are included so the teardown consumer can resolve
	// the engine key after the record is removed.
	err = s.pool.QueryRow(ctx, `
		SELECT mb.local, d.name, mb.password_hash, mb.quota_bytes
		FROM mailboxes mb JOIN domains d ON d.id = mb.domain_id
		WHERE mb.id=$1`, id).Scan(&local, &domain, &passwordHash, &quotaBytes)
	return
}

// MarkProvisioned records the outcome of a mailbox provisioning attempt.
func (s *Service) MarkProvisioned(ctx context.Context, id, status, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE mailboxes SET engine_status=$2, provisioning_error=$3, last_provisioned_at=now(), updated_at=now()
		WHERE id=$1`, id, status, errMsg)
	return err
}

// AliasTarget resolves the data an alias provisioning consumer needs.
func (s *Service) AliasTarget(ctx context.Context, id string) (source, destination string, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT al.source || '@' || d.name, al.destination FROM aliases al JOIN domains d ON d.id=al.domain_id WHERE al.id=$1 AND al.deleted_at IS NULL`, id).Scan(&source, &destination)
	return
}

// MarkAliasProvisioned records a successful alias provisioning attempt.
func (s *Service) MarkAliasProvisioned(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE aliases SET last_provisioned_at=now() WHERE id=$1`, id)
	return err
}

type rowScanner interface{ Scan(...any) error }

func scanMailbox(row rowScanner) (*Mailbox, error) {
	var m Mailbox
	if err := row.Scan(&m.ID, &m.DomainID, &m.Local, &m.Address, &m.DisplayName,
		&m.QuotaBytes, &m.Status, &m.EngineStatus, &m.PasswordSetAt, &m.CreatedAt); err != nil {
		return nil, err
	}
	return &m, nil
}

// ListByDomain returns mailboxes for a domain.
func (s *Service) ListByDomain(ctx context.Context, domainID string) ([]*Mailbox, error) {
	rows, err := httpapi.PageRows(ctx, s.pool, mailboxColsJoined, "mailboxes mb JOIN domains d ON d.id=mb.domain_id", "mb.domain_id=$1 AND mb.deleted_at IS NULL AND d.deleted_at IS NULL", "mb.local,mb.id", []string{"mb.local", "mb.display_name"}, map[string]string{"name": "mb.local", "local": "mb.local", "createdAt": "mb.created_at", "status": "mb.status"}, "mb.status", domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Mailbox
	for rows.Next() {
		m, err := scanMailbox(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// EmailByID looks up a mailbox and its domain name (used by provisioning).
func (s *Service) EmailByID(ctx context.Context, id string) (address, domain string, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT mb.local || '@' || d.name, d.name
		FROM mailboxes mb JOIN domains d ON d.id = mb.domain_id
		WHERE mb.id = $1 AND mb.deleted_at IS NULL`, id).Scan(&address, &domain)
	return
}

// Create maintains the database record (hashing the password once) and enqueues
// provisioning when a live engine is configured. Plaintext never leaves memory.
func (s *Service) Create(ctx context.Context, domainID, local, displayName string, quotaBytes int64, password string) (*Mailbox, error) {
	// Resolve the domain first so a missing domain is a clean 404.
	var domainName string
	err := s.pool.QueryRow(ctx, `
		SELECT name FROM domains WHERE id=$1 AND deleted_at IS NULL AND status <> 'suspended'`, domainID).Scan(&domainName)
	if err != nil {
		return nil, errNotFound
	}
	passwordHash := ""
	if password != "" {
		if passwordHash, err = auth.HashPassword(ctx, password); err != nil {
			return nil, err
		}
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO mailboxes (domain_id, local, display_name, quota_bytes, status, engine_status, password_set_at, password_hash)
		VALUES ($1,$2,$3,$4,'active', $5, CASE WHEN $6 <> '' THEN now() ELSE NULL END, $7)
		RETURNING `+mailboxColsPlain,
		domainID, local, displayName, quotaBytes, engineStatusLabel(s.engine), password, passwordHash)
	mb, err := scanMailbox(row)
	if err != nil {
		return nil, err
	}
	// Durable outbox trigger queues provisioning atomically.
	return mb, nil
}

// Update changes display name, quota, and optionally resets the password. A new
// password is hashed once and provisioning is enqueued when an engine is live.
func (s *Service) Update(ctx context.Context, id, displayName string, quotaBytes int64, password string) (*Mailbox, error) {
	if password != "" {
		passwordHash, err := auth.HashPassword(ctx, password)
		if err != nil {
			return nil, err
		}
		row := s.pool.QueryRow(ctx, `
			UPDATE mailboxes SET display_name=$2, quota_bytes=$3,
				password_hash=$4, password_set_at=now(), updated_at=now()
			WHERE id=$1 AND deleted_at IS NULL
			RETURNING `+mailboxColsPlain, id, displayName, quotaBytes, passwordHash)
		mb, err := scanMailbox(row)
		if err != nil {
			return nil, errNotFound
		}
		// Durable outbox trigger queues provisioning atomically.
		return mb, nil
	}
	row := s.pool.QueryRow(ctx, `
		UPDATE mailboxes SET display_name=$2, quota_bytes=$3, updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL
		RETURNING `+mailboxColsPlain, id, displayName, quotaBytes)
	return scanMailbox(row)
}

// Delete soft-deletes a mailbox and enqueues removal on the engine.
func (s *Service) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE mailboxes SET deleted_at=now(), status='suspended', updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil || tag.RowsAffected() == 0 {
		return errNotFound
	}
	// Durable outbox trigger queues teardown atomically.
	return nil
}

func engineStatusLabel(e engine.Engine) string {
	if e.Configured() {
		return "provisioning"
	}
	return "unmanaged"
}

// Alias is the API representation of a forwarding alias.
type Alias struct {
	ID          string `json:"id"`
	DomainID    string `json:"domainId"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Forwarding  bool   `json:"forwarding"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt"`
}

const aliasColumns = `al.id::text, al.domain_id::text, al.source, al.destination, al.forwarding, al.enabled,
	to_char(al.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

func scanAlias(row rowScanner) (*Alias, error) {
	var a Alias
	if err := row.Scan(&a.ID, &a.DomainID, &a.Source, &a.Destination, &a.Forwarding, &a.Enabled, &a.CreatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

// ListAliases returns aliases for a domain.
func (s *Service) ListAliases(ctx context.Context, domainID string) ([]*Alias, error) {
	rows, err := httpapi.PageRows(ctx, s.pool, aliasColumns, "aliases al", "al.domain_id=$1 AND al.deleted_at IS NULL", "al.source,al.id", []string{"al.source", "al.destination"}, map[string]string{"name": "al.source", "source": "al.source", "createdAt": "al.created_at"}, "al.enabled", domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Alias
	for rows.Next() {
		a, err := scanAlias(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateAlias inserts an alias for a domain and enqueues provisioning.
func (s *Service) CreateAlias(ctx context.Context, domainID, source, destination string, forwarding bool) (*Alias, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO aliases AS al (domain_id, source, destination, forwarding, enabled)
		VALUES ($1,$2,$3,$4,true)
		RETURNING `+aliasColumns, domainID, strings.ToLower(source), destination, forwarding)
	a, err := scanAlias(row)
	if err != nil {
		return nil, err
	}
	// Durable outbox trigger queues provisioning atomically.
	return a, nil
}

// DeleteAlias removes an alias and enqueues teardown on the engine.
func (s *Service) DeleteAlias(ctx context.Context, id string) error {
	var source string
	if err := s.pool.QueryRow(ctx,
		`SELECT source FROM aliases WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&source); err != nil {
		return errNotFound
	}
	tag, err := s.pool.Exec(ctx, `UPDATE aliases SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil || tag.RowsAffected() == 0 {
		return errNotFound
	}
	// Durable outbox trigger queues teardown atomically.
	return nil
}

var errNotFound = errors.New("not found")

// Handler exposes the mailboxes API.
type Handler struct {
	svc *Service
}

// NewHandler builds the mailbox HTTP handler.
func NewHandler(pool *pgxpool.Pool, rec *audit.Recorder, eng engine.Engine, enqueue Enqueuer) *Handler {
	return &Handler{svc: NewService(pool, rec, eng, enqueue)}
}

// Service exposes the underlying service for provisioning consumers.
func (h *Handler) Service() *Service { return h.svc }

func (h *Handler) validateLocal(local string) map[string]any {
	local = strings.ToLower(strings.TrimSpace(local))
	if local == "" || len(local) > 64 {
		return map[string]any{"local": "Local part is required and must be 64 characters or fewer."}
	}
	for _, r := range local {
		if !(r == '.' || r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return map[string]any{"local": "Local part may contain only letters, digits, dots, hyphens and underscores."}
		}
	}
	return nil
}

func (h *Handler) validateDestination(addr string) map[string]any {
	if !validAddr(addr) {
		return map[string]any{"destination": "Enter a valid destination address."}
	}
	return nil
}

func validAddr(addr string) bool {
	parsed, err := mail.ParseAddress(addr)
	return err == nil && parsed.Address == addr
}

// RegisterRoutes attaches the mailboxes/aliases API to the router group
// behind auth. Static routes are registered before parameterized ones.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	r.GET("/mailboxes/engine", auth, h.EngineStatus)
	mailboxes := r.Group("/domains/:id", auth)
	mailboxes.GET("/mailboxes", h.List)
	mailboxes.POST("/mailboxes", h.Create)
	mailboxes.GET("/aliases", h.ListAliases)
	mailboxes.POST("/aliases", h.CreateAlias)
	r.PATCH("/mailboxes/:mailbox_id", auth, h.Update)
	r.GET("/mailboxes/:mailbox_id/provisioning", auth, h.Provisioning)
	r.DELETE("/mailboxes/:mailbox_id", auth, h.Delete)
	r.DELETE("/aliases/:alias_id", auth, h.DeleteAlias)
}

// List handles GET /api/v1/domains/:id/mailboxes.
func (h *Handler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	all, err := h.svc.ListByDomain(ctx, c.Param("id"))
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

type mailboxRequest struct {
	Local       string `json:"local"`
	DisplayName string `json:"displayName"`
	QuotaBytes  int64  `json:"quotaBytes"`
	Password    string `json:"password"`
}

// Create handles POST /api/v1/domains/:id/mailboxes.
func (h *Handler) Create(c *gin.Context) {
	var req mailboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	fields := map[string]any{}
	req.Local = strings.ToLower(strings.TrimSpace(req.Local))
	if f := h.validateLocal(req.Local); f != nil {
		for k, v := range f {
			fields[k] = v
		}
	}
	if len(req.Password) < 12 {
		fields["password"] = "Password must be at least 12 characters."
	}
	if req.QuotaBytes < 0 {
		fields["quotaBytes"] = "Quota cannot be negative."
	}
	if len(fields) > 0 {
		httpapi.FailFields(c, "Check the highlighted fields and try again.", fields)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	mb, err := h.svc.Create(ctx, c.Param("id"), req.Local, req.DisplayName, req.QuotaBytes, req.Password)
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Domain not found or suspended.")
			return
		}
		if isUniqueViolation(err) {
			httpapi.Fail(c, http.StatusConflict, httpapi.ErrConflict, "A mailbox with this address already exists.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "mailbox.create", "mailbox", mb.ID,
		gin.H{"address": mb.Address}, c.ClientIP())
	if !h.svc.engine.Configured() {
		c.JSON(http.StatusCreated, gin.H{"data": mb, "warning": "Mailbox created as a record; no mail engine is configured for live provisioning."})
		return
	}
	httpapi.Respond(c, http.StatusCreated, mb)
}

// Update handles PATCH /api/v1/mailboxes/:id.
func (h *Handler) Update(c *gin.Context) {
	var req mailboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	if req.Password != "" && len(req.Password) < 12 {
		httpapi.FailFields(c, "Password must be at least 12 characters.", map[string]any{"password": "Password must be at least 12 characters."})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if req.QuotaBytes < 0 {
		httpapi.FailFields(c, "Quota cannot be negative.", nil)
		return
	}
	mb, err := h.svc.Update(ctx, c.Param("mailbox_id"), req.DisplayName, req.QuotaBytes, req.Password)
	if err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Mailbox not found.")
		return
	}
	lvl := "mailbox.update"
	if req.Password != "" {
		lvl = "mailbox.set_password"
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), lvl, "mailbox", mb.ID, nil, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, mb)
}

// Delete handles DELETE /api/v1/mailboxes/:id.
func (h *Handler) Delete(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Delete(ctx, c.Param("mailbox_id")); err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Mailbox not found.")
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "mailbox.delete", "mailbox", c.Param("mailbox_id"), nil, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, gin.H{"ok": true})
}

// EngineStatus handles GET /api/v1/mailboxes/engine.
func (h *Handler) EngineStatus(c *gin.Context) {
	httpapi.Respond(c, http.StatusOK, h.svc.EngineStatus())
}

// ListAliases handles GET /api/v1/domains/:id/aliases.
func (h *Handler) ListAliases(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	all, err := h.svc.ListAliases(ctx, c.Param("id"))
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

type aliasRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Forwarding  bool   `json:"forwarding"`
}

// CreateAlias handles POST /api/v1/domains/:id/aliases.
func (h *Handler) CreateAlias(c *gin.Context) {
	var req aliasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	fields := map[string]any{}
	req.Source = strings.ToLower(strings.TrimSpace(req.Source))
	if f := h.validateLocal(req.Source); f != nil {
		for k, v := range f {
			fields[k] = v
		}
	}
	if f := h.validateDestination(req.Destination); f != nil {
		for k, v := range f {
			fields[k] = v
		}
	}
	if req.Source == req.Destination {
		fields["destination"] = "Destination must differ from the alias source."
	}
	if len(fields) > 0 {
		httpapi.FailFields(c, "Check the highlighted fields and try again.", fields)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	a, err := h.svc.CreateAlias(ctx, c.Param("id"), req.Source, req.Destination, req.Forwarding)
	if err != nil {
		if isUniqueViolation(err) {
			httpapi.Fail(c, http.StatusConflict, httpapi.ErrConflict, "An alias with this source already exists.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "alias.create", "alias", a.ID,
		gin.H{"source": a.Source, "destination": a.Destination}, c.ClientIP())
	httpapi.Respond(c, http.StatusCreated, a)
}

// DeleteAlias handles DELETE /api/v1/aliases/:id.
func (h *Handler) DeleteAlias(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.DeleteAlias(ctx, c.Param("alias_id")); err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Alias not found.")
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "alias.delete", "alias", c.Param("alias_id"), nil, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, gin.H{"ok": true})
}

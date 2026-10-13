// Package domains manages mail domains, ownership verification via DNS TXT
// challenge, and the DNS records DevMail expects to exist for mail delivery.
package domains

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/audit"
	"github.com/Euphrase8/DevMail/backend/internal/auth"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/Euphrase8/DevMail/backend/internal/netutil"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Domain is the API representation of a mail domain under management.
type Domain struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	ServerID          string  `json:"serverId"`
	Status            string  `json:"status"`
	VerificationToken string  `json:"verificationToken"`
	VerificationName  string  `json:"verificationName"`
	VerifiedAt        string  `json:"verifiedAt"`
	CatchAll          bool    `json:"catchAll"`
	QuotaBytes        int64   `json:"quotaBytes"`
	CreatedAt         string  `json:"createdAt"`
	CheckResults      []Check `json:"checkResults,omitempty"`
}

// Check is one DNS validation result for a domain.
type Check struct {
	Kind      string `json:"kind"`
	OK        bool   `json:"ok"`
	Detail    string `json:"detail"`
	CheckedAt string `json:"checkedAt"`
}

// VerificationName is the DNS TXT name used for ownership proof.
const VerificationName = "devmail-verification"

const domainColumns = `id::text, name,
	COALESCE(server_id::text,''), status, verification_token,
	COALESCE(to_char(verified_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
	catch_all, quota_bytes,
	to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

// Enqueuer schedules an asynchronous provisioning job.
type Enqueuer func(ctx context.Context, kind string, payload map[string]string) error

// Service implements domain persistence and DNS validation.
type Service struct {
	pool    *pgxpool.Pool
	audit   *audit.Recorder
	enqueue Enqueuer
}

// NewService creates the domain service.
func NewService(pool *pgxpool.Pool, auditRec *audit.Recorder, enqueue Enqueuer) *Service {
	return &Service{pool: pool, audit: auditRec, enqueue: enqueue}
}

func (s *Service) enqueueProvision(ctx context.Context, kind, id string) {
	if s.enqueue != nil {
		_ = s.enqueue(ctx, kind, map[string]string{"domainId": id})
	}
}

type rowScanner interface{ Scan(...any) error }

func scanDomain(row rowScanner) (*Domain, error) {
	var d Domain
	if err := row.Scan(&d.ID, &d.Name, &d.ServerID, &d.Status, &d.VerificationToken,
		&d.VerifiedAt, &d.CatchAll, &d.QuotaBytes, &d.CreatedAt); err != nil {
		return nil, err
	}
	if d.VerificationToken != "" {
		d.VerificationName = fmt.Sprintf("_%s.%s", VerificationName, d.Name)
	}
	return &d, nil
}

// List returns all non-deleted domains.
func (s *Service) List(ctx context.Context) ([]*Domain, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+domainColumns+` FROM domains WHERE deleted_at IS NULL ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Domain
	for rows.Next() {
		d, err := scanDomain(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Get returns a single domain by id.
func (s *Service) Get(ctx context.Context, id string) (*Domain, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+domainColumns+` FROM domains WHERE id=$1 AND deleted_at IS NULL`, id)
	return scanDomain(row)
}

// Name resolves a domain name by id, including soft-deleted rows, for the
// teardown path.
func (s *Service) Name(ctx context.Context, id string) (string, error) {
	var name string
	err := s.pool.QueryRow(ctx, `SELECT name FROM domains WHERE id=$1`, id).Scan(&name)
	return name, err
}

// MarkProvisioned records the outcome of a domain provisioning attempt.
func (s *Service) MarkProvisioned(ctx context.Context, id, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE domains SET last_provisioned_at=now(), provisioning_error=$2, updated_at=now()
		WHERE id=$1`, id, errMsg)
	return err
}

// Create inserts a new domain, assigning it a unique TXT verification token.
func (s *Service) Create(ctx context.Context, name, serverID string, quotaBytes int64, catchAll bool) (*Domain, error) {
	token, err := newVerificationToken()
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO domains (name, server_id, status, verification_token, quota_bytes, catch_all)
		VALUES ($1, $2, 'pending', $3, $4, $5)
		RETURNING `+domainColumns,
		name, nullableUUID(serverID), token, quotaBytes, catchAll)
	return scanDomain(row)
}

func nullableUUID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func newVerificationToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Update edits a domain's server assignment, catch-all flag and quota.
func (s *Service) Update(ctx context.Context, id, serverID string, catchAll bool, quotaBytes int64) (*Domain, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE domains SET server_id=$2, catch_all=$3, quota_bytes=$4, updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL
		RETURNING `+domainColumns,
		id, nullableUUID(serverID), catchAll, quotaBytes)
	return scanDomain(row)
}

// Delete soft-deletes a domain.
func (s *Service) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE domains SET deleted_at=now(), status='suspended' WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	s.enqueueProvision(ctx, "domain.remove", id)
	return nil
}

// Validate runs the full set of DNS checks for a domain and stores results:
//  1. ownership TXT token present at _devmail-verification.<domain>
//  2. MX records exist and resolve
//  3. SPF record (or equivalent) exists
//  4. DMARC record exists
//  5. DKIM selector TXT present (default selector)
func (s *Service) Validate(ctx context.Context, id string) (*Domain, error) {
	d, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.Status != "verified" {
		return nil, errors.New("domain must be verified before running mail DNS checks")
	}
	results := []Check{}
	resolveTimeout := 8 * time.Second

	ownerFound := false
	txts, txtErr := netutil.LookupTXT(ctx, "_"+VerificationName+"."+d.Name, resolveTimeout)
	if txtErr == nil {
		for _, t := range txts {
			for _, part := range strings.Fields(strings.ToLower(t)) {
				if strings.HasPrefix(part, "dmverify=") && part == "dmverify="+d.VerificationToken {
					ownerFound = true
				}
			}
		}
	}
	results = append(results, Check{
		Kind: "ownership", OK: ownerFound,
		Detail:    detailOf(ownerFound, "Ownership TXT token published", "Token missing or mismatch"),
		CheckedAt: nowUTC(),
	})

	mx, mxErr := netutil.LookupMX(ctx, d.Name, resolveTimeout)
	mxOK := mxErr == nil && len(mx) > 0
	results = append(results, Check{Kind: "mx", OK: mxOK,
		Detail: func() string {
			if !mxOK {
				return detailOf(false, "", "No MX records found")
			}
			return fmt.Sprintf("%d MX record(s): %s", len(mx), strings.Join(mx, ", "))
		}(), CheckedAt: nowUTC()})

	spfFound := false
	spfList, spfErr := netutil.LookupTXT(ctx, d.Name, resolveTimeout)
	if spfErr == nil {
		for _, t := range spfList {
			if strings.Contains(strings.ToLower(t), "v=spf1") {
				spfFound = true
			}
		}
	}
	results = append(results, Check{Kind: "spf", OK: spfFound,
		Detail:    detailOf(spfFound, "SPF record present", "No SPF record found"),
		CheckedAt: nowUTC()})

	dmarcFound := false
	dmarcList, dmarcErr := netutil.LookupTXT(ctx, "_dmarc."+d.Name, resolveTimeout)
	if dmarcErr == nil {
		for _, t := range dmarcList {
			if strings.Contains(strings.ToLower(t), "v=dmarc1") {
				dmarcFound = true
			}
		}
	}
	results = append(results, Check{Kind: "dmarc", OK: dmarcFound,
		Detail:    detailOf(dmarcFound, "DMARC record present", "No DMARC record found"),
		CheckedAt: nowUTC()})

	dkimFound := false
	dkimList, dkimErr := netutil.LookupTXT(ctx, "default._domainkey."+d.Name, resolveTimeout)
	if dkimErr == nil {
		for _, t := range dkimList {
			if strings.Contains(strings.ToLower(t), "v=dkim1") || strings.Contains(strings.ToLower(t), "p=") {
				dkimFound = true
			}
		}
	}
	results = append(results, Check{Kind: "dkim", OK: dkimFound,
		Detail:    detailOf(dkimFound, "DKIM selector 'default' present", "No DKIM record for selector 'default'"),
		CheckedAt: nowUTC()})

	allOK := true
	var stored []map[string]any
	for _, r := range results {
		ok := r.OK
		allOK = allOK && ok
		stored = append(stored, map[string]any{"kind": r.Kind, "ok": r.OK, "detail": r.Detail, "checked_at": r.CheckedAt})
		_, _ = s.pool.Exec(ctx, `
			INSERT INTO domain_check_results (domain_id, kind, ok, detail, checked_at)
			VALUES ($1,$2,$3,jsonb_build_object('detail',$4),now())`,
			id, r.Kind, r.OK, r.Detail)
	}
	_ = stored

	newStatus := d.Status
	if allOK {
		newStatus = "active"
	} else {
		newStatus = "degraded"
	}
	_, _ = s.pool.Exec(ctx, `UPDATE domains SET status=$2, updated_at=now() WHERE id=$1`, id, newStatus)

	d.Status = newStatus
	d.CheckResults = results
	return d, nil
}

// CheckHistory returns stored DNS validation results for a domain, newest
// first, optionally filtered by check kind.
func (s *Service) CheckHistory(ctx context.Context, id, kind string, limit int) ([]Check, int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if _, err := s.Get(ctx, id); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM domain_check_results WHERE domain_id=$1 AND ($2='' OR kind=$2)`, id, kind).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT kind, ok, COALESCE(detail->>'detail',''),
		       to_char(checked_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM domain_check_results
		WHERE domain_id=$1 AND ($2='' OR kind=$2)
		ORDER BY checked_at DESC LIMIT $3`, id, kind, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Check
	for rows.Next() {
		var c Check
		if err := rows.Scan(&c.Kind, &c.OK, &c.Detail, &c.CheckedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// Verify checks the ownership TXT token and marks the domain verified.
func (s *Service) Verify(ctx context.Context, id string) (*Domain, error) {
	d, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	ownerFound := false
	timeout := 8 * time.Second
	txts, txtErr := netutil.LookupTXT(ctx, "_"+VerificationName+"."+d.Name, timeout)
	if txtErr == nil {
		for _, t := range txts {
			for _, part := range strings.Fields(strings.ToLower(t)) {
				if part == "dmverify="+d.VerificationToken {
					ownerFound = true
				}
			}
		}
	}
	if !ownerFound {
		return nil, errors.New("verification token not found in DNS")
	}
	_, _ = s.pool.Exec(ctx,
		`UPDATE domains SET status='verified', verified_at=now(), updated_at=now() WHERE id=$1`, id)
	row := s.pool.QueryRow(ctx, `SELECT `+domainColumns+` FROM domains WHERE id=$1`, id)
	d, err = scanDomain(row)
	if err != nil {
		return nil, err
	}
	s.enqueueProvision(ctx, "domain.provision", id)
	return d, nil
}

func detailOf(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

var errNotFound = errors.New("not found")

// Handler exposes domains over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds the domain HTTP handler.
func NewHandler(pool *pgxpool.Pool, auditRec *audit.Recorder, enqueue Enqueuer) *Handler {
	return &Handler{svc: NewService(pool, auditRec, enqueue)}
}

// Service exposes the underlying service for provisioning consumers.
func (h *Handler) Service() *Service { return h.svc }

type domainRequest struct {
	Name       string `json:"name"`
	ServerID   string `json:"serverId"`
	QuotaBytes int64  `json:"quotaBytes"`
	CatchAll   bool   `json:"catchAll"`
}

func (h *Handler) validate(req *domainRequest) map[string]any {
	fields := map[string]any{}
	name := strings.ToLower(strings.TrimSpace(req.Name))
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		fields["name"] = "Enter a fully qualified domain name (e.g. example.com)."
		return fields
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			fields["name"] = "Domain labels must be 1-63 characters."
			return fields
		}
		for _, r := range label {
			if !(r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
				fields["name"] = "Domain labels may contain only letters, digits and hyphens."
				return fields
			}
		}
	}
	if req.QuotaBytes < 0 {
		fields["quotaBytes"] = "Quota cannot be negative."
	}
	return fields
}

// RegisterRoutes attaches the domains API to the router group behind auth.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	domains := r.Group("/domains", auth)
	domains.GET("", h.List)
	domains.POST("", h.Create)
	domains.POST("/:id/verify", h.Verify)
	domains.POST("/:id/validate", h.Validate)
	domains.POST("/:id/provision", h.Provision)
	domains.GET("/:id/checks", h.Checks)
	domains.PATCH("/:id", h.Update)
	domains.DELETE("/:id", h.Delete)
}

// List handles GET /api/v1/domains.
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

// Create handles POST /api/v1/domains.
func (h *Handler) Create(c *gin.Context) {
	var req domainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	if fields := h.validate(&req); len(fields) > 0 {
		httpapi.FailFields(c, "Check the highlighted fields and try again.", fields)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	d, err := h.svc.Create(ctx, req.Name, req.ServerID, req.QuotaBytes, req.CatchAll)
	if err != nil {
		if isUniqueViolation(err) {
			httpapi.Fail(c, http.StatusConflict, httpapi.ErrConflict, "This domain is already managed.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "domain.create", "domain", d.ID, gin.H{"name": d.Name}, c.ClientIP())
	httpapi.Respond(c, http.StatusCreated, d)
}

// Verify handles POST /api/v1/domains/:id/verify.
func (h *Handler) Verify(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	d, err := h.svc.Verify(ctx, c.Param("id"))
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Domain not found.")
			return
		}
		httpapi.Fail(c, http.StatusConflict, httpapi.ErrConflict, "Verification failed. "+err.Error())
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "domain.verify", "domain", d.ID, nil, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, d)
}

// Validate handles POST /api/v1/domains/:id/validate.
func (h *Handler) Validate(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	d, err := h.svc.Validate(ctx, c.Param("id"))
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Domain not found.")
			return
		}
		httpapi.Fail(c, http.StatusConflict, httpapi.ErrConflict, err.Error())
		return
	}
	httpapi.Respond(c, http.StatusOK, d)
}

// Checks handles GET /api/v1/domains/:id/checks?kind=&limit=.
func (h *Handler) Checks(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		n := 0
		ok := len(v) > 0
		for _, r := range v {
			if r < '0' || r > '9' {
				ok = false
				break
			}
			n = n*10 + int(r-'0')
		}
		if ok && n > 0 {
			limit = n
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	checks, total, err := h.svc.CheckHistory(ctx, c.Param("id"), c.Query("kind"), limit)
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Domain not found.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	data := make([]any, len(checks))
	for i := range checks {
		data[i] = checks[i]
	}
	httpapi.RespondList(c, data, 1, limit, total)
}

// Delete handles DELETE /api/v1/domains/:id.
func (h *Handler) Delete(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Delete(ctx, c.Param("id")); err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Domain not found.")
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "domain.delete", "domain", c.Param("id"), nil, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, gin.H{"ok": true})
}

// Update handles PATCH /api/v1/domains/:id.
func (h *Handler) Update(c *gin.Context) {
	var req domainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Fail(c, http.StatusBadRequest, httpapi.ErrBadRequest, "Invalid request body.")
		return
	}
	if req.QuotaBytes < 0 {
		httpapi.FailFields(c, "Quota cannot be negative.", map[string]any{"quotaBytes": "Quota cannot be negative."})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	d, err := h.svc.Update(ctx, c.Param("id"), req.ServerID, req.CatchAll, req.QuotaBytes)
	if err != nil {
		httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Domain not found.")
		return
	}
	httpapi.Respond(c, http.StatusOK, d)
}

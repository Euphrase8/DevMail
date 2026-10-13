// Package dns generates the DNS records a managed mail domain requires, tracks
// their publication status, and applies them through a configured provider.
package dns

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/audit"
	"github.com/Euphrase8/DevMail/backend/internal/auth"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/Euphrase8/DevMail/backend/internal/providers"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Record is a planned or applied DNS record for a domain.
type Record struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Value     string `json:"value"`
	TTL       int    `json:"ttl"`
	Priority  int    `json:"priority"`
	Purpose   string `json:"purpose"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
	AppliedAt string `json:"appliedAt"`
}

const recordColumns = `id::text, type, name, value, ttl, priority, purpose, status, detail,
	COALESCE(to_char(applied_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),'')`

type rowScanner interface{ Scan(...any) error }

func scanRecord(row rowScanner) (*Record, error) {
	var r Record
	if err := row.Scan(&r.ID, &r.Type, &r.Name, &r.Value, &r.TTL, &r.Priority, &r.Purpose, &r.Status, &r.Detail, &r.AppliedAt); err != nil {
		return nil, err
	}
	return &r, nil
}

// Service computes and applies DNS records.
type Service struct {
	pool       *pgxpool.Pool
	audit      *audit.Recorder
	providers  *providers.Service
	passphrase string
}

// NewService creates the DNS service.
func NewService(pool *pgxpool.Pool, rec *audit.Recorder, prov *providers.Service, passphrase string) *Service {
	return &Service{pool: pool, audit: rec, providers: prov, passphrase: passphrase}
}

var errNotFound = errors.New("not found")

type domainFacts struct {
	name              string
	verificationToken string
	dkimSelector      string
	dkimPublicKey     string
	serverID          string
}

func (s *Service) loadFacts(ctx context.Context, domainID string) (*domainFacts, error) {
	d := &domainFacts{}
	var serverID *string
	err := s.pool.QueryRow(ctx, `
		SELECT name, verification_token, dkim_selector, dkim_public_key, server_id::text
		FROM domains WHERE id=$1 AND deleted_at IS NULL`, domainID).
		Scan(&d.name, &d.verificationToken, &d.dkimSelector, &d.dkimPublicKey, &serverID)
	if err != nil {
		return nil, errNotFound
	}
	if serverID != nil {
		d.serverID = *serverID
	}
	return d, nil
}

type serverFacts struct {
	hostname string
	ipv4     string
}

func (s *Service) loadServer(ctx context.Context, id string) serverFacts {
	if id == "" {
		return serverFacts{}
	}
	var sf serverFacts
	_ = s.pool.QueryRow(ctx, `
		SELECT hostname, COALESCE(host(ipv4),'') FROM servers WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&sf.hostname, &sf.ipv4)
	return sf
}

// plan builds the desired record set from real configuration.
func plan(d *domainFacts, srv serverFacts) []Record {
	mailHost := srv.hostname
	if mailHost == "" {
		mailHost = "mail." + d.name
	}
	records := []Record{
		{Type: "TXT", Name: "_devmail-verification." + d.name, Value: "dmverify=" + d.verificationToken,
			TTL: 3600, Purpose: "ownership"},
		{Type: "MX", Name: d.name, Value: mailHost, TTL: 3600, Priority: 10, Purpose: "mx"},
		{Type: "TXT", Name: d.name, Value: "v=spf1 mx -all", TTL: 3600, Purpose: "spf"},
		{Type: "TXT", Name: "_dmarc." + d.name,
			Value: "v=DMARC1; p=quarantine; rua=mailto:postmaster@" + d.name + "; fo=1",
			TTL:   3600, Purpose: "dmarc"},
		{Type: "PTR", Name: mailHost, Value: mailHost, TTL: 3600, Purpose: "ptr"},
	}
	if d.dkimPublicKey != "" {
		selector := d.dkimSelector
		if selector == "" {
			selector = "default"
		}
		records = append(records, Record{
			Type: "TXT", Name: selector + "._domainkey." + d.name,
			Value: "v=DKIM1; k=rsa; p=" + d.dkimPublicKey, TTL: 3600, Purpose: "dkim",
		})
	}
	if srv.ipv4 != "" {
		records = append(records, Record{Type: "A", Name: mailHost, Value: srv.ipv4, TTL: 3600, Purpose: "mail-address"})
	}
	return records
}

// Plan computes the record set, upserts it, and returns the persisted rows.
func (s *Service) Plan(ctx context.Context, domainID string) ([]*Record, error) {
	facts, err := s.loadFacts(ctx, domainID)
	if err != nil {
		return nil, err
	}
	srv := s.loadServer(ctx, facts.serverID)
	for _, r := range plan(facts, srv) {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO dns_records (domain_id, type, name, value, ttl, priority, purpose)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (domain_id, type, name) DO UPDATE SET
				value = EXCLUDED.value,
				ttl = EXCLUDED.ttl,
				priority = EXCLUDED.priority,
				purpose = EXCLUDED.purpose,
				status = CASE WHEN dns_records.value <> EXCLUDED.value THEN 'planned' ELSE dns_records.status END,
				updated_at = now()`,
			domainID, r.Type, r.Name, r.Value, r.TTL, r.Priority, r.Purpose)
		if err != nil {
			return nil, err
		}
	}
	return s.list(ctx, domainID)
}

func (s *Service) list(ctx context.Context, domainID string) ([]*Record, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+recordColumns+` FROM dns_records
		WHERE domain_id=$1 ORDER BY purpose, type, name`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Apply pushes every unpublished record through the given provider.
func (s *Service) Apply(ctx context.Context, domainID, providerID string) ([]*Record, []providers.Result, error) {
	if providerID == "" {
		return nil, nil, errors.New("a provider must be selected to apply records")
	}
	adapter, _, err := s.providers.Adapter(ctx, providerID)
	if err != nil {
		return nil, nil, errors.New("selected provider was not found")
	}
	pending, err := s.pending(ctx, domainID)
	if err != nil {
		return nil, nil, err
	}
	if len(pending) == 0 {
		rows, _ := s.list(ctx, domainID)
		return rows, nil, nil
	}
	req := make([]providers.Record, 0, len(pending))
	for _, r := range pending {
		req = append(req, providers.Record{Type: r.Type, Name: r.Name, Value: r.Value, TTL: r.TTL, Priority: r.Priority})
	}
	results, err := adapter.Apply(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	byName := map[string]providers.Result{}
	for _, res := range results {
		byName[res.Type+"|"+res.Name] = res
	}
	for _, r := range pending {
		res, ok := byName[r.Type+"|"+r.Name]
		if !ok {
			continue
		}
		_, _ = s.pool.Exec(ctx, `
			UPDATE dns_records SET status=$3, detail=$4, provider_id=$5::uuid,
				applied_at = CASE WHEN $3 IN ('published','manual') THEN now() ELSE applied_at END,
				updated_at = now()
			WHERE domain_id=$1 AND type=$6 AND name=$2`,
			domainID, r.Name, res.Status, res.Detail, providerID, r.Type)
	}
	rows, err := s.list(ctx, domainID)
	return rows, results, err
}

func (s *Service) pending(ctx context.Context, domainID string) ([]*Record, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+recordColumns+` FROM dns_records
		WHERE domain_id=$1 AND status IN ('planned','error') ORDER BY purpose, type, name`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Handler exposes the DNS API.
type Handler struct {
	svc *Service
}

// NewHandler builds the DNS HTTP handler.
func NewHandler(pool *pgxpool.Pool, rec *audit.Recorder, prov *providers.Service, passphrase string) *Handler {
	return &Handler{svc: NewService(pool, rec, prov, passphrase)}
}

// Service exposes the underlying service for provisioning consumers.
func (h *Handler) Service() *Service { return h.svc }

// RegisterRoutes attaches the DNS API to the router group behind auth.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	g := r.Group("/domains/:id", auth)
	g.GET("/dns-records", h.Plan)
	g.POST("/dns-records/apply", h.Apply)
	g.POST("/dkim", h.GenerateDKIM)
}

// Plan handles GET /api/v1/domains/:id/dns-records.
func (h *Handler) Plan(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	records, err := h.svc.Plan(ctx, c.Param("id"))
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Domain not found.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	data := make([]any, len(records))
	for i := range records {
		data[i] = records[i]
	}
	httpapi.RespondList(c, data, 1, len(records), len(records))
}

// Apply handles POST /api/v1/domains/:id/dns-records/apply.
func (h *Handler) Apply(c *gin.Context) {
	var req struct {
		ProviderID string `json:"providerId"`
	}
	_ = c.ShouldBindJSON(&req)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	records, results, err := h.svc.Apply(ctx, c.Param("id"), req.ProviderID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Domain not found.")
			return
		}
		httpapi.Fail(c, http.StatusConflict, httpapi.ErrConflict, err.Error())
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "domain.dns_apply", "domain", c.Param("id"),
		gin.H{"providerId": req.ProviderID, "records": len(records)}, c.ClientIP())
	data := make([]any, len(records))
	for i := range records {
		data[i] = records[i]
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "results": results})
}

// GenerateDKIM handles POST /api/v1/domains/:id/dkim.
func (h *Handler) GenerateDKIM(c *gin.Context) {
	var req struct {
		Selector string `json:"selector"`
	}
	_ = c.ShouldBindJSON(&req)
	selector := strings.TrimSpace(req.Selector)
	if selector == "" {
		selector = "default"
	}
	if !validSelector(selector) {
		httpapi.FailFields(c, "Invalid DKIM selector.", map[string]any{"selector": "Use letters, digits and hyphens only."})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	pub, record, err := h.svc.GenerateDKIM(ctx, c.Param("id"), selector)
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpapi.Fail(c, http.StatusNotFound, httpapi.ErrNotFound, "Domain not found.")
			return
		}
		httpapi.InternalError(c)
		return
	}
	h.svc.audit.Record(ctx, auth.CurrentUser(c), "domain.dkim_generate", "domain", c.Param("id"),
		gin.H{"selector": selector}, c.ClientIP())
	httpapi.Respond(c, http.StatusOK, gin.H{"selector": selector, "publicKey": pub, "record": record})
}

func validSelector(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for _, r := range s {
		if !(r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

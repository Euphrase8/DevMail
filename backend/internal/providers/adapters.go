package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// manualAdapter describes records the operator publishes by hand. It never
// contacts a network and reports every record as "manual".
type manualAdapter struct{}

func (manualAdapter) Kind() string                 { return "manual" }
func (manualAdapter) Verify(context.Context) error { return nil }
func (manualAdapter) Apply(_ context.Context, records []Record) ([]Result, error) {
	out := make([]Result, 0, len(records))
	for _, r := range records {
		out = append(out, Result{Type: r.Type, Name: r.Name, Status: "manual",
			Detail: "Add this record manually at your DNS host."})
	}
	return out, nil
}

// cloudflareAdapter talks to the Cloudflare v4 API with a zone-scoped token.
// Only the fixed api.cloudflare.com host is contacted; user input never
// becomes a shell command or an arbitrary URL.
type cloudflareAdapter struct {
	zone     string
	token    string
	client   *http.Client
	zoneID   string
	zoneName string
}

func (c *cloudflareAdapter) Kind() string { return "cloudflare" }

const cloudflareBase = "https://api.cloudflare.com/client/v4"

type cfEnvelope struct {
	Success bool            `json:"success"`
	Errors  []cfError       `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cfRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
}

func (c *cloudflareAdapter) do(ctx context.Context, method, path string, body any) (*cfEnvelope, error) {
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, cloudflareBase+path, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare request failed: %w", err)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, 1<<20)
	var env cfEnvelope
	if err := json.NewDecoder(limited).Decode(&env); err != nil {
		return nil, fmt.Errorf("cloudflare returned an unreadable response")
	}
	if !env.Success {
		if len(env.Errors) > 0 {
			return &env, fmt.Errorf("cloudflare error %d: %s", env.Errors[0].Code, env.Errors[0].Message)
		}
		return &env, fmt.Errorf("cloudflare rejected the request (status %d)", resp.StatusCode)
	}
	return &env, nil
}

func (c *cloudflareAdapter) Verify(ctx context.Context) error {
	if c.token == "" {
		return fmt.Errorf("no token is stored")
	}
	if _, err := c.do(ctx, http.MethodGet, "/user/tokens/verify", nil); err != nil {
		return err
	}
	_, err := c.resolveZone(ctx)
	return err
}

// resolveZone determines the zone id from an explicit id or a zone name lookup.
func (c *cloudflareAdapter) resolveZone(ctx context.Context) (string, error) {
	if c.zoneID != "" {
		return c.zoneID, nil
	}
	if c.zone == "" {
		return "", fmt.Errorf("no zone is configured for this provider")
	}
	if isHex32(c.zone) {
		c.zoneID = c.zone
		return c.zoneID, nil
	}
	env, err := c.do(ctx, http.MethodGet, "/zones?name="+url.QueryEscape(c.zone), nil)
	if err != nil {
		return "", err
	}
	var zones []cfRecord
	if err := json.Unmarshal(env.Result, &zones); err != nil || len(zones) == 0 {
		return "", fmt.Errorf("zone %q was not found on this account", c.zone)
	}
	c.zoneID = zones[0].ID
	c.zoneName = zones[0].Name
	return c.zoneID, nil
}

func (c *cloudflareAdapter) Apply(ctx context.Context, records []Record) ([]Result, error) {
	zoneID, err := c.resolveZone(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(records))
	for _, r := range records {
		res := Result{Type: r.Type, Name: r.Name}
		existing, ferr := c.findRecord(ctx, zoneID, r)
		if ferr != nil {
			res.Status = "error"
			res.Detail = ferr.Error()
			out = append(out, res)
			continue
		}
		body := cfRecord{Type: r.Type, Name: r.Name, Content: r.Value, TTL: ttlOr(r.TTL)}
		if r.Type == "MX" {
			prio := r.Priority
			body.Priority = &prio
		}
		var perr error
		if existing != nil {
			_, perr = c.do(ctx, http.MethodPatch, "/zones/"+zoneID+"/dns_records/"+existing.ID, body)
		} else {
			_, perr = c.do(ctx, http.MethodPost, "/zones/"+zoneID+"/dns_records", body)
		}
		if perr != nil {
			res.Status = "error"
			res.Detail = perr.Error()
		} else {
			res.Status = "published"
			res.Detail = "Published to Cloudflare."
		}
		out = append(out, res)
	}
	return out, nil
}

func (c *cloudflareAdapter) findRecord(ctx context.Context, zoneID string, r Record) (*cfRecord, error) {
	path := "/zones/" + zoneID + "/dns_records?type=" + url.QueryEscape(r.Type) + "&name=" + url.QueryEscape(r.Name)
	env, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var found []cfRecord
	if err := json.Unmarshal(env.Result, &found); err != nil {
		return nil, nil
	}
	if len(found) == 0 {
		return nil, nil
	}
	return &found[0], nil
}

func ttlOr(ttl int) int {
	if ttl <= 0 {
		return 3600
	}
	return ttl
}

func isHex32(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, r := range strings.ToLower(s) {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

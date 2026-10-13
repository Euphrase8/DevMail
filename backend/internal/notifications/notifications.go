package notifications

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/Euphrase8/DevMail/backend/internal/netutil"
	"github.com/Euphrase8/DevMail/backend/internal/secretbox"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dispatcher interface {
	Dispatch(context.Context, string, []byte) error
}
type Webhook struct{ AllowPrivate bool }

func Validate(m map[string]any) error {
	raw, err := httpapi.RequiredString(m, "url")
	if err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Fragment != "" {
		return errors.New("a valid HTTP(S) webhook URL without user credentials is required")
	}
	return nil
}
func (w Webhook) Dispatch(ctx context.Context, endpoint string, body []byte) error {
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := netutil.ResolveSafe(ctx, host, w.AllowPrivate)
		if err != nil {
			return nil, err
		}
		d := net.Dialer{Timeout: 5 * time.Second}
		return d.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}, TLSHandshakeTimeout: 5 * time.Second}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("invalid webhook request")
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		if errors.Is(err, netutil.ErrNetworkBlocked) {
			return netutil.ErrNetworkBlocked
		}
		return errors.New("webhook connection failed")
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", res.StatusCode)
	}
	return nil
}
func Register(v1 *gin.RouterGroup, pool *pgxpool.Pool, key string, dispatcher Dispatcher, middleware ...gin.HandlerFunc) {
	resource := &httpapi.Resource{Pool: pool, Table: "notification_channels", Validate: Validate, Transform: func(m, old map[string]any) error {
		raw := m["url"].(string)
		sealed, err := secretbox.Seal(key, raw)
		if err != nil {
			return err
		}
		m["endpoint"] = sealed
		u, _ := url.Parse(raw)
		m["host"] = u.Host
		delete(m, "url")
		return nil
	}, Redact: func(m map[string]any) { delete(m, "endpoint") }}
	resource.Register(v1, "/notifications", middleware...)
	g := v1.Group("/notifications", middleware...)
	g.POST("/:id/test", func(c *gin.Context) {
		item, err := resource.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			httpapi.Fail(c, 404, httpapi.ErrNotFound, "Channel not found.")
			return
		}
		if !item.Enabled {
			httpapi.Fail(c, 409, httpapi.ErrConflict, "Channel is disabled.")
			return
		}
		endpoint, _ := item.Config["endpoint"].(string)
		endpoint, err = secretbox.Open(key, endpoint)
		if err == nil {
			err = dispatcher.Dispatch(c.Request.Context(), endpoint, []byte(`{"event":"devmail.test","message":"Webhook verification"}`))
		}
		status, detail := "succeeded", "Delivered"
		if err != nil {
			status = "failed"
			detail = err.Error()
		}
		_, saveErr := pool.Exec(c.Request.Context(), `INSERT INTO notification_events(channel_id,status,detail) VALUES($1,$2,$3)`, item.ID, status, detail)
		if saveErr != nil {
			httpapi.InternalError(c)
			return
		}
		httpapi.Respond(c, 200, gin.H{"status": status, "detail": strings.TrimSpace(detail)})
	})
	g.GET("/:id/events", func(c *gin.Context) {
		rows, err := pool.Query(c.Request.Context(), `SELECT id::text,status,detail,created_at FROM notification_events WHERE channel_id=$1 ORDER BY created_at DESC LIMIT 100`, c.Param("id"))
		if err != nil {
			httpapi.InternalError(c)
			return
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, status, detail string
			var at time.Time
			if rows.Scan(&id, &status, &detail, &at) != nil {
				httpapi.InternalError(c)
				return
			}
			out = append(out, gin.H{"id": id, "status": status, "detail": detail, "createdAt": at})
		}
		httpapi.Respond(c, 200, out)
	})
}

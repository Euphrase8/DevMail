package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Resource centralizes persistence/envelopes while each domain owns validation and actions.
type Resource struct {
	Pool      *pgxpool.Pool
	Table     string // Trusted compile-time table name only.
	Validate  func(map[string]any) error
	Redact    func(map[string]any)
	Transform func(map[string]any, map[string]any) error
}
type ResourceItem struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Enabled   bool           `json:"enabled"`
	Config    map[string]any `json:"config"`
	CreatedAt time.Time      `json:"createdAt"`
}

func (r *Resource) Get(ctx context.Context, id string) (*ResourceItem, error) {
	var item ResourceItem
	var raw []byte
	err := r.Pool.QueryRow(ctx, `SELECT id::text,name,enabled,config,created_at FROM `+r.Table+` WHERE id=$1`, id).Scan(&item.ID, &item.Name, &item.Enabled, &raw, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(raw, &item.Config)
	return &item, err
}
func (r *Resource) Register(group *gin.RouterGroup, path string, middleware ...gin.HandlerFunc) {
	g := group.Group(path, middleware...)
	g.GET("", r.List)
	g.POST("", r.Save)
	g.PATCH("/:id", r.Save)
	g.DELETE("/:id", r.Delete)
}
func (r *Resource) List(c *gin.Context) {
	rows, err := r.Pool.Query(c.Request.Context(), `SELECT id::text,name,enabled,config,created_at FROM `+r.Table+` ORDER BY created_at DESC,id`)
	if err != nil {
		InternalError(c)
		return
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		var item ResourceItem
		var raw []byte
		if err = rows.Scan(&item.ID, &item.Name, &item.Enabled, &raw, &item.CreatedAt); err != nil {
			InternalError(c)
			return
		}
		if err = json.Unmarshal(raw, &item.Config); err != nil {
			InternalError(c)
			return
		}
		if r.Redact != nil {
			r.Redact(item.Config)
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		InternalError(c)
		return
	}
	RespondList(c, items, 1, len(items), len(items))
}
func (r *Resource) Save(c *gin.Context) {
	var req ResourceItem
	if c.ShouldBindJSON(&req) != nil {
		Fail(c, 400, ErrBadRequest, "Invalid request body.")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 200 || req.Config == nil {
		FailFields(c, "A name and configuration are required.", nil)
		return
	}
	var old map[string]any
	if c.Param("id") != "" {
		item, err := r.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			Fail(c, 404, ErrNotFound, "Record not found.")
			return
		}
		old = item.Config
	}
	if r.Validate != nil {
		if err := r.Validate(req.Config); err != nil {
			FailFields(c, err.Error(), nil)
			return
		}
	}
	if r.Transform != nil {
		if err := r.Transform(req.Config, old); err != nil {
			InternalError(c)
			return
		}
	}
	raw, err := json.Marshal(req.Config)
	if err != nil {
		InternalError(c)
		return
	}
	id := c.Param("id")
	status := 200
	if id == "" {
		status = 201
		err = r.Pool.QueryRow(c.Request.Context(), `INSERT INTO `+r.Table+`(name,enabled,config) VALUES($1,$2,$3) RETURNING id::text`, req.Name, req.Enabled, raw).Scan(&id)
	} else {
		_, err = r.Pool.Exec(c.Request.Context(), `UPDATE `+r.Table+` SET name=$2,enabled=$3,config=$4,updated_at=now() WHERE id=$1`, id, req.Name, req.Enabled, raw)
	}
	if err != nil {
		InternalError(c)
		return
	}
	item, err := r.Get(c.Request.Context(), id)
	if err != nil {
		InternalError(c)
		return
	}
	if r.Redact != nil {
		r.Redact(item.Config)
	}
	Respond(c, status, item)
}
func (r *Resource) Delete(c *gin.Context) {
	tag, err := r.Pool.Exec(c.Request.Context(), `DELETE FROM `+r.Table+` WHERE id=$1`, c.Param("id"))
	if err != nil {
		InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		Fail(c, 404, ErrNotFound, "Record not found.")
		return
	}
	Respond(c, 200, gin.H{"ok": true})
}
func RequiredString(m map[string]any, key string) (string, error) {
	v, ok := m[key].(string)
	if !ok || strings.TrimSpace(v) == "" {
		return "", errors.New(key + " is required")
	}
	return v, nil
}

package backups

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Adapter interface {
	Run(context.Context, string) (int64, string, error)
}

// PGDump only executes an administrator-selected binary. User input never becomes commands or paths.
type PGDump struct{ Binary, Directory, DatabaseURL string }

func (p PGDump) Run(ctx context.Context, id string) (int64, string, error) {
	if p.Binary == "" || p.Directory == "" {
		return 0, "", errors.New("backup adapter is not configured")
	}
	cfg, err := pgx.ParseConfig(p.DatabaseURL)
	if err != nil {
		return 0, "", errors.New("invalid backup database configuration")
	}
	if err = os.MkdirAll(p.Directory, 0700); err != nil {
		return 0, "", errors.New("backup directory is unavailable")
	}
	file, err := os.CreateTemp(p.Directory, "backup-*.dump")
	if err != nil {
		return 0, "", errors.New("cannot create backup file")
	}
	path := file.Name()
	complete := false
	defer func() {
		file.Close()
		if !complete {
			os.Remove(path)
		}
	}()
	cmd := exec.CommandContext(ctx, p.Binary, "--format=custom", "--no-owner", "--no-acl")
	cmd.Env = append(os.Environ(), "PGHOST="+cfg.Host, "PGPORT="+strconv.Itoa(int(cfg.Port)), "PGUSER="+cfg.User, "PGPASSWORD="+cfg.Password, "PGDATABASE="+cfg.Database)
	if ssl, ok := cfg.RuntimeParams["sslmode"]; ok {
		cmd.Env = append(cmd.Env, "PGSSLMODE="+ssl)
	}
	cmd.Stdout = file
	cmd.Stderr = io.Discard
	if err = cmd.Run(); err != nil {
		return 0, "", errors.New("database backup failed")
	}
	if err = file.Sync(); err != nil {
		return 0, "", errors.New("backup flush failed")
	}
	if _, err = file.Seek(0, 0); err != nil {
		return 0, "", err
	}
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, "", err
	}
	complete = true
	return size, filepath.Base(path) + " sha256=" + hex.EncodeToString(hash.Sum(nil)), nil
}
func Validate(m map[string]any) error {
	if m["kind"] != "postgres" {
		return errors.New("only the postgres backup adapter is available")
	}
	return nil
}
func Register(v1 *gin.RouterGroup, pool *pgxpool.Pool, adapter Adapter, middleware ...gin.HandlerFunc) {
	resource := &httpapi.Resource{Pool: pool, Table: "backup_targets", Validate: Validate}
	resource.Register(v1, "/backups", middleware...)
	g := v1.Group("/backups", middleware...)
	g.POST("/:id/run", func(c *gin.Context) {
		target, err := resource.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			httpapi.Fail(c, 404, httpapi.ErrNotFound, "Backup target not found.")
			return
		}
		if !target.Enabled {
			httpapi.Fail(c, 409, httpapi.ErrConflict, "Backup target is disabled.")
			return
		}
		var id string
		err = pool.QueryRow(c.Request.Context(), `INSERT INTO backup_runs(target_id,status) VALUES($1,'running') RETURNING id::text`, target.ID).Scan(&id)
		if err != nil {
			httpapi.InternalError(c)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
		defer cancel()
		size, detail, err := adapter.Run(ctx, id)
		status := "succeeded"
		if err != nil {
			status = "failed"
			detail = err.Error()
		}
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer saveCancel()
		_, err = pool.Exec(saveCtx, `UPDATE backup_runs SET status=$2,detail=$3,bytes=$4,finished_at=now() WHERE id=$1`, id, status, detail, size)
		if err != nil {
			httpapi.InternalError(c)
			return
		}
		httpapi.Respond(c, 200, gin.H{"id": id, "status": status, "detail": detail, "bytes": size})
	})
	g.GET("/:id/runs", func(c *gin.Context) {
		rows, err := pool.Query(c.Request.Context(), `SELECT id::text,status,detail,bytes,created_at FROM backup_runs WHERE target_id=$1 ORDER BY created_at DESC LIMIT 100`, c.Param("id"))
		if err != nil {
			httpapi.InternalError(c)
			return
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, status, detail string
			var size int64
			var at time.Time
			if rows.Scan(&id, &status, &detail, &size, &at) != nil {
				httpapi.InternalError(c)
				return
			}
			out = append(out, gin.H{"id": id, "status": status, "detail": detail, "bytes": size, "createdAt": at})
		}
		httpapi.Respond(c, 200, out)
	})
}

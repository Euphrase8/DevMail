package db

import (
	"context"
	"fmt"
	"github.com/Euphrase8/DevMail/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

func Rollback(ctx context.Context, pool *pgxpool.Pool, steps int) error {
	if steps < 1 || steps > 100 {
		return fmt.Errorf("steps must be between 1 and 100")
	}
	for i := 0; i < steps; i++ {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(810003)`); err != nil {
			tx.Rollback(ctx)
			return err
		}
		var name string
		err = tx.QueryRow(ctx, `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1 FOR UPDATE`).Scan(&name)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		file := strings.TrimSuffix(name, ".sql") + ".down.sql"
		raw, err := migrations.FS.ReadFile(file)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		if _, err = tx.Exec(ctx, string(raw)); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version=$1`, name); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

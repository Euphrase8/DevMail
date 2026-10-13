package mailboxes

import (
	"context"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
)

func (s *Service) Deleted(ctx context.Context, id string) bool {
	var deleted bool
	_ = s.pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM mailboxes WHERE id=$1`, id).Scan(&deleted)
	return deleted
}
func (s *Service) AliasDeleted(ctx context.Context, id string) bool {
	var deleted bool
	_ = s.pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM aliases WHERE id=$1`, id).Scan(&deleted)
	return deleted
}

func (h *Handler) Provisioning(c *gin.Context) {
	var status, detail string
	var at *string
	err := h.svc.pool.QueryRow(c.Request.Context(), `SELECT engine_status, provisioning_error, last_provisioned_at::text FROM mailboxes WHERE id=$1 AND deleted_at IS NULL`, c.Param("mailbox_id")).Scan(&status, &detail, &at)
	if err != nil {
		httpapi.Fail(c, 404, httpapi.ErrNotFound, "Mailbox not found.")
		return
	}
	httpapi.Respond(c, 200, gin.H{"status": status, "error": detail, "lastProvisionedAt": at})
}

// Keep record and queue writes in a single statement transaction via an outbox trigger.
func (s *Service) configureQueue(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO settings(key,value) VALUES ('provisioning_enabled',to_jsonb($1::boolean)) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, s.engine.Configured())
	return err
}

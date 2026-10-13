package auth

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/Euphrase8/DevMail/backend/internal/secretbox"
	"github.com/gin-gonic/gin"
)

func (h *Handler) checkPassword(c *gin.Context, password string) bool {
	var hash string
	if h.pool.QueryRow(c.Request.Context(), `SELECT password_hash FROM users WHERE id=$1`, CurrentUser(c).ID).Scan(&hash) != nil || !VerifyPassword(hash, password) {
		httpapi.Fail(c, 401, httpapi.ErrUnauthorized, "Current password is incorrect.")
		return false
	}
	return true
}
func (h *Handler) Enroll(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&req) != nil {
		httpapi.Fail(c, 400, httpapi.ErrBadRequest, "Invalid request.")
		return
	}
	if !h.checkPassword(c, req.Password) {
		return
	}
	var enabled bool
	_ = h.pool.QueryRow(c.Request.Context(), `SELECT totp_enabled FROM users WHERE id=$1`, CurrentUser(c).ID).Scan(&enabled)
	if enabled {
		httpapi.Fail(c, 409, httpapi.ErrConflict, "MFA is already enabled.")
		return
	}
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		httpapi.InternalError(c)
		return
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
	sealed, err := secretbox.Seal(h.key, secret)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	_, err = h.pool.Exec(c.Request.Context(), `UPDATE users SET totp_pending=$2,totp_pending_until=now()+interval '10 minutes' WHERE id=$1`, CurrentUser(c).ID, sealed)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	uri := "otpauth://totp/DevMail:" + url.PathEscape(CurrentUser(c).Email) + "?secret=" + secret + "&issuer=DevMail&algorithm=SHA1&digits=6&period=30"
	httpapi.Respond(c, 200, gin.H{"secret": secret, "uri": uri})
}
func (h *Handler) VerifyEnrollment(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if c.ShouldBindJSON(&req) != nil {
		httpapi.Fail(c, 400, httpapi.ErrBadRequest, "Invalid request.")
		return
	}
	var sealed string
	err := h.pool.QueryRow(c.Request.Context(), `SELECT totp_pending FROM users WHERE id=$1 AND totp_pending_until>now() AND totp_enabled=false`, CurrentUser(c).ID).Scan(&sealed)
	secret, openErr := secretbox.Open(h.key, sealed)
	step, ok := matchingStep(secret, req.Code, time.Now())
	if err != nil || openErr != nil || !ok {
		httpapi.Fail(c, 422, httpapi.ErrValidation, "Invalid or expired enrollment code.")
		return
	}
	codes, hashes := []string{}, []string{}
	for i := 0; i < 10; i++ {
		code, e := NewSessionToken()
		if e != nil {
			httpapi.InternalError(c)
			return
		}
		code = code[:20]
		codes = append(codes, code)
		hashes = append(hashes, HashSessionToken(code))
	}
	tag, err := h.pool.Exec(c.Request.Context(), `UPDATE users SET totp_secret=totp_pending,totp_pending=NULL,totp_pending_until=NULL,totp_enabled=true,totp_last_step=$2,recovery_hashes=$3 WHERE id=$1 AND totp_enabled=false AND totp_pending=$4`, CurrentUser(c).ID, step, hashes, sealed)
	if err != nil || tag.RowsAffected() != 1 {
		httpapi.Fail(c, 409, httpapi.ErrConflict, "Enrollment changed; start again.")
		return
	}
	AuthEvent(c.Request.Context(), h.pool, CurrentUser(c), "auth.mfa_enabled", c.ClientIP())
	httpapi.Respond(c, 200, gin.H{"recoveryCodes": codes})
}
func (s *Service) ConsumeMFA(ctx context.Context, id, code string) error {
	var sealed string
	var enabled bool
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(totp_secret,''),totp_enabled FROM users WHERE id=$1`, id).Scan(&sealed, &enabled)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	secret, err := secretbox.Open(s.key, sealed)
	if err != nil {
		return errors.New("MFA unavailable")
	}
	if step, ok := matchingStep(secret, code, time.Now()); ok {
		tag, err := s.pool.Exec(ctx, `UPDATE users SET totp_last_step=$2 WHERE id=$1 AND totp_last_step<$2`, id, step)
		if err == nil && tag.RowsAffected() == 1 {
			return nil
		}
	}
	hash := HashSessionToken(strings.TrimSpace(code))
	tag, err := s.pool.Exec(ctx, `UPDATE users SET recovery_hashes=array_remove(recovery_hashes,$2) WHERE id=$1 AND $2=ANY(recovery_hashes)`, id, hash)
	if err == nil && tag.RowsAffected() == 1 {
		return nil
	}
	return errors.New("invalid MFA code")
}
func (h *Handler) DisableMFA(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if c.ShouldBindJSON(&req) != nil {
		httpapi.Fail(c, 400, httpapi.ErrBadRequest, "Invalid request.")
		return
	}
	if !h.checkPassword(c, req.Password) {
		return
	}
	if h.svc.ConsumeMFA(c.Request.Context(), CurrentUser(c).ID, req.Code) != nil {
		httpapi.Fail(c, 401, httpapi.ErrUnauthorized, "Invalid MFA code.")
		return
	}
	_, err := h.pool.Exec(c.Request.Context(), `UPDATE users SET totp_enabled=false,totp_secret=NULL,recovery_hashes='{}',totp_last_step=-1,totp_pending=NULL WHERE id=$1`, CurrentUser(c).ID)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	AuthEvent(c.Request.Context(), h.pool, CurrentUser(c), "auth.mfa_disabled", c.ClientIP())
	httpapi.Respond(c, 200, gin.H{"ok": true})
}
func (h *Handler) Password(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if c.ShouldBindJSON(&req) != nil {
		httpapi.Fail(c, 400, httpapi.ErrBadRequest, "Invalid request.")
		return
	}
	if len(req.NewPassword) < 12 {
		httpapi.FailFields(c, "Use at least 12 characters.", nil)
		return
	}
	if !h.checkPassword(c, req.CurrentPassword) {
		return
	}
	hash, err := HashPassword(c.Request.Context(), req.NewPassword)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	defer tx.Rollback(c.Request.Context())
	_, err = tx.Exec(c.Request.Context(), `UPDATE users SET password_hash=$2,updated_at=now() WHERE id=$1`, CurrentUser(c).ID, hash)
	if err == nil {
		_, err = tx.Exec(c.Request.Context(), `UPDATE sessions SET revoked=true WHERE user_id=$1`, CurrentUser(c).ID)
	}
	if err != nil || tx.Commit(c.Request.Context()) != nil {
		httpapi.InternalError(c)
		return
	}
	AuthEvent(c.Request.Context(), h.pool, CurrentUser(c), "auth.password_changed", c.ClientIP())
	httpapi.Respond(c, 200, gin.H{"ok": true})
}
func (h *Handler) Sessions(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `SELECT id::text,ip,user_agent,created_at,expires_at,last_seen_at FROM sessions WHERE user_id=$1 AND NOT revoked AND expires_at>now() ORDER BY created_at DESC`, CurrentUser(c).ID)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, ip, agent string
		var created, expires, seen time.Time
		if rows.Scan(&id, &ip, &agent, &created, &expires, &seen) != nil {
			httpapi.InternalError(c)
			return
		}
		out = append(out, gin.H{"id": id, "ip": ip, "userAgent": agent, "createdAt": created, "expiresAt": expires, "lastSeenAt": seen})
	}
	httpapi.Respond(c, 200, out)
}
func (h *Handler) RevokeSessions(c *gin.Context) {
	_, err := h.pool.Exec(c.Request.Context(), `UPDATE sessions SET revoked=true WHERE user_id=$1`, CurrentUser(c).ID)
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	AuthEvent(c.Request.Context(), h.pool, CurrentUser(c), "auth.sessions_revoked", c.ClientIP())
	httpapi.Respond(c, 200, gin.H{"ok": true})
}
func (h *Handler) Rotate(c *gin.Context) {
	old, _ := c.Cookie(SessionCookie)
	token, err := NewSessionToken()
	if err != nil {
		httpapi.InternalError(c)
		return
	}
	tag, err := h.pool.Exec(c.Request.Context(), `UPDATE sessions SET token_hash=$2 WHERE token_hash=$1 AND NOT revoked AND expires_at>now()`, HashSessionToken(old), HashSessionToken(token))
	if err != nil || tag.RowsAffected() != 1 {
		httpapi.Fail(c, 401, httpapi.ErrUnauthorized, "Session expired.")
		return
	}
	h.cookie(c, token, int(SessionLifetime.Seconds()))
	httpapi.Respond(c, 200, gin.H{"ok": true})
}
func (h *Handler) cookie(c *gin.Context, token string, age int) {
	c.SetSameSite(2)
	c.SetCookie(SessionCookie, token, age, "/", "", h.secure, true)
}

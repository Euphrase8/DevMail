// Package api assembles the complete HTTP router, wiring every bounded domain
// module into the /api/v1 route tree. Keeping assembly here (rather than in
// internal/httpapi, which holds generic plumbing) avoids package import cycles.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/audit"
	"github.com/Euphrase8/DevMail/backend/internal/auth"
	"github.com/Euphrase8/DevMail/backend/internal/config"
	"github.com/Euphrase8/DevMail/backend/internal/dns"
	"github.com/Euphrase8/DevMail/backend/internal/domains"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/Euphrase8/DevMail/backend/internal/jobs"
	"github.com/Euphrase8/DevMail/backend/internal/mailboxes"
	"github.com/Euphrase8/DevMail/backend/internal/monitoring"
	"github.com/Euphrase8/DevMail/backend/internal/providers"
	"github.com/Euphrase8/DevMail/backend/internal/servers"
	"github.com/Euphrase8/DevMail/backend/internal/settings"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Options wires the router to its dependencies.
type Options struct {
	Pool       *pgxpool.Pool
	Config     *config.Config
	Logger     *slog.Logger
	MailEngine mailboxes.Engine
}

// NewRouter assembles the full API. The mail-engine status route reports
// "not_configured" rather than pretending provisioning is live.
func NewRouter(opts Options) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(httpapi.RecoverMiddleware(opts.Logger), httpapi.RequestIDMiddleware(opts.Logger),
		httpapi.SecurityHeadersMiddleware(), httpapi.CORSMiddleware(opts.Config.AllowedOrigins))
	_ = r.SetTrustedProxies(nil)

	rec := audit.NewRecorder(opts.Pool)
	authSvc := auth.NewService(opts.Pool)
	authMw := auth.RequireAuth(authSvc)

	v1 := r.Group("/api/v1")

	// Public health + L7 checks.
	v1.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		db := "ok"
		if err := opts.Pool.Ping(ctx); err != nil {
			db = "error"
		}
		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"status":  "ok",
			"db":      db,
			"time":    time.Now().UTC().Format(time.RFC3339),
			"version": "0.1.0",
		}})
	})

	// Auth module owns its own public + protected routes.
	auth.Register(v1, opts.Pool, authMw)

	// Admin resources: everything under /api/v1 is authenticated.
	servers.NewHandler(opts.Pool, rec, opts.Config.AllowPrivateNetworks).RegisterRoutes(v1, authMw)
	domains.NewHandler(opts.Pool, rec).RegisterRoutes(v1, authMw)
	mailboxes.NewHandler(opts.Pool, rec, opts.MailEngine).RegisterRoutes(v1, authMw)
	providersHandler := providers.NewHandler(opts.Pool, opts.Config.SecretKey, rec)
	providersHandler.RegisterRoutes(v1, authMw)
	dns.NewHandler(opts.Pool, rec, providersHandler.Service(), opts.Config.SecretKey).RegisterRoutes(v1, authMw)
	settings.NewHandler(opts.Pool).RegisterRoutes(v1, authMw)
	monitoring.NewHandler(opts.Pool).RegisterRoutes(v1, authMw)
	audit.NewHandler(opts.Pool).RegisterRoutes(v1, authMw)
	jobs.NewHandler(opts.Pool, opts.Logger, rec).RegisterRoutes(v1, authMw)

	return r
}
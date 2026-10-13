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
	"github.com/Euphrase8/DevMail/backend/internal/backups"
	"github.com/Euphrase8/DevMail/backend/internal/config"
	"github.com/Euphrase8/DevMail/backend/internal/dns"
	"github.com/Euphrase8/DevMail/backend/internal/domains"
	"github.com/Euphrase8/DevMail/backend/internal/engine"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/Euphrase8/DevMail/backend/internal/jobs"
	"github.com/Euphrase8/DevMail/backend/internal/mailboxes"
	"github.com/Euphrase8/DevMail/backend/internal/monitoring"
	"github.com/Euphrase8/DevMail/backend/internal/notifications"
	"github.com/Euphrase8/DevMail/backend/internal/providers"
	"github.com/Euphrase8/DevMail/backend/internal/routing"
	"github.com/Euphrase8/DevMail/backend/internal/servers"
	"github.com/Euphrase8/DevMail/backend/internal/settings"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Options wires the router to its dependencies.
type Options struct {
	Context    context.Context
	Pool       *pgxpool.Pool
	Config     *config.Config
	Logger     *slog.Logger
	MailEngine engine.Engine
}

// NewRouter assembles the full API. The mail-engine status route reports
// "not_configured" rather than pretending provisioning is live.
func NewRouter(opts Options) *gin.Engine {
	if opts.MailEngine == nil {
		opts.MailEngine = engine.Noop{}
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(httpapi.RecoverMiddleware(opts.Logger), httpapi.RequestIDMiddleware(opts.Logger),
		httpapi.SecurityHeadersMiddleware(), httpapi.CORSMiddleware(opts.Config.AllowedOrigins))
	_ = r.SetTrustedProxies(nil)

	rec := audit.NewRecorder(opts.Pool)
	authSvc := auth.NewService(opts.Pool)
	authMw := auth.RequireAuth(authSvc)

	v1 := r.Group("/api/v1")

	v1.Use(auth.CSRF(opts.Config.SecretKey, !opts.Config.IsDevelopment()))
	v1.GET("/auth/csrf", func(c *gin.Context) {})
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
	auth.Register(v1, opts.Pool, authMw, opts.Config.SecretKey, !opts.Config.IsDevelopment())

	// One jobs service is shared by the HTTP handler and the provisioning
	// consumers so both operate on the same registry and worker.
	jobsSvc := jobs.NewService(opts.Pool, opts.Logger)
	var enqueue func(context.Context, string, map[string]string) error
	if opts.MailEngine != nil && opts.MailEngine.Configured() {
		enqueue = func(ctx context.Context, kind string, payload map[string]string) error {
			_, err := jobsSvc.Enqueue(ctx, kind, payload)
			return err
		}
	}

	// Admin resources: everything under /api/v1 is authenticated.
	servers.NewHandler(opts.Pool, rec, opts.Config.AllowPrivateNetworks).RegisterRoutes(v1, authMw)
	domainsHandler := domains.NewHandler(opts.Pool, rec, enqueue)
	domainsHandler.RegisterRoutes(v1, authMw)
	mailboxesHandler := mailboxes.NewHandler(opts.Pool, rec, opts.MailEngine, enqueue)
	mailboxesHandler.RegisterRoutes(v1, authMw)
	_, err := opts.Pool.Exec(context.Background(), `INSERT INTO settings(key,value) VALUES ('provisioning_enabled',to_jsonb($1::boolean)) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, opts.MailEngine.Configured())
	if err != nil {
		panic("cannot configure durable provisioning")
	}
	providersHandler := providers.NewHandler(opts.Pool, opts.Config.SecretKey, rec)
	providersHandler.RegisterRoutes(v1, authMw)
	dnsHandler := dns.NewHandler(opts.Pool, rec, providersHandler.Service(), opts.Config.SecretKey)
	dnsHandler.RegisterRoutes(v1, authMw)
	settings.NewHandler(opts.Pool).RegisterRoutes(v1, authMw)
	monitoring.NewHandler(opts.Pool).RegisterRoutes(v1, authMw)
	audit.NewHandler(opts.Pool).RegisterRoutes(v1, authMw)
	jobs.NewHandler(jobsSvc, rec).RegisterRoutes(v1, authMw)
	routing.Register(v1, opts.Pool, authMw, auth.RequireRole(auth.RoleDomainAdmin))
	backups.Register(v1, opts.Pool, backups.PGDump{Binary: opts.Config.PGDumpPath, Directory: opts.Config.BackupDirectory, DatabaseURL: opts.Config.DatabaseURL}, authMw, auth.RequireRole(auth.RoleSuperAdmin))
	notifications.Register(v1, opts.Pool, opts.Config.SecretKey, notifications.Webhook{AllowPrivate: opts.Config.AllowPrivateNetworks}, authMw, auth.RequireRole(auth.RoleServerAdmin))

	registerProvisioningConsumers(jobsSvc, opts.MailEngine, mailboxesHandler.Service(),
		domainsHandler.Service(), dnsHandler.Service())
	if opts.Context != nil {
		jobsSvc.Start(opts.Context)
	}

	return r
}

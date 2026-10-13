# DevMail application index

The deliverable is `frontend/` (TanStack Start, React, TypeScript) connected to
`backend/` (Go, Gin, PostgreSQL) through `/api/v1`. `ROADMAP.md` tracks completion.

## Backend

- `cmd/api/main.go`: configuration, migrations, engine selection, HTTP lifecycle.
- `internal/api/router.go`: route assembly; `provisioning.go`: job consumers.
- `internal/config`, `db`, `httpapi`: environment, forward/rollback migration runner,
  errors, request contracts, pagination, filtering and middleware.
- `internal/auth`: users, roles, passwords, cookies, sessions.
- `internal/servers`: server registry, maintenance, guarded connectivity, health history.
- `internal/domains`: ownership challenges, DNS validation, check history.
- `internal/mailboxes`: domain-scoped mailbox and alias records and provisioning.
- `internal/engine`: SQL virtual domain/mailbox/alias adapter; unconfigured adapter.
- `internal/jobs`: persistent queue, consumers, retries, cancellation.
- `internal/dns`, `providers`, `secretbox`: DNS plans, DKIM, provider adapters, encryption.
- `internal/settings`, `audit`, `monitoring`: configuration, events, dashboard counts,
  readiness and Prometheus-style request metrics.
- `migrations`: versioned embedded forward and rollback SQL schema changes.

## Frontend

- `src/lib/api.ts`: typed backend client and error envelopes.
- `src/router.tsx`, `routes/__root.tsx`: Query provider and root routing.
- `src/routes`: overview, servers, domains, DNS, providers, mailboxes, aliases,
  routing, backups, notifications, jobs, monitoring, audit, users, settings, webmail,
  docs, login. Each route delegates to `src/components/mail`.
- `components/mail/store.tsx`: connectivity, session, shared shell state.
- `components/mail/shell`: navigation, account, workspace, search.
- `components/mail/resources`: live server, domain, mailbox and alias management.
- `components/mail/{dns,providers,jobs}`: live API screens.
- `components/ui`: reusable shadcn/Radix primitives.
- `src/lib/mail/data.ts`: navigation and page metadata only; application data comes from the API.
- `src/test`: routing and interaction checks.
- `vite.config.ts`: frontend server on 8080, `/api` proxy to API on 7000.
- `Dockerfile`, `../compose.yaml`, `../deploy/nginx.conf`: production container images,
  reverse-proxy configuration and local stack definition.

## Verification

Backend: `go test ./...`, `go vet ./...`. Frontend: `npm run lint`, `npm test`,
`npx tsc --noEmit`, `npm run build`. Live checks must use a running PostgreSQL
database and actual authenticated HTTP requests. Adapter success only confirms
the adapter's operation; external mail delivery needs an installed mail server.

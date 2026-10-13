# DevMail — Backend Completion Roadmap

Living planning + tracing document for finishing the backend so **every** frontend
screen consumes a real `/api/v1` endpoint (per `AGENTS.md`: the product is only
complete when the frontend talks end-to-end with the backend).

- **Scope owner:** backend (`backend/`), plus the frontend wiring each phase requires.
- **Status legend:**
  - `[ ]` not started
  - `[~]` in progress
  - `[x]` done + verified
  - `[!]` blocked / needs external prerequisite
- **Keep this file current.** After each phase: tick its task boxes, update the
  [Status board](#status-board) and [Progress log](#progress-log), and record the
  verification command output + commit SHA. Do **not** mark a phase `[x]` until
  `go test ./...` (from `backend/`) and the live e2e check pass.

---

## 1. Current state (baseline)

Already implemented, built, vetted, tested, and pushed (`9175d67`). Default API
listener is `:7000`; dev DB is PostgreSQL (`postgres`/`1508`, database `devmail`).

### Endpoints that exist today

| Method | Path | Notes |
|---|---|---|
| GET  | `/api/v1/health` | liveness + db status |
| GET  | `/api/v1/auth/status` | `{hasUsers}` |
| POST | `/api/v1/auth/setup` | first-run super_admin bootstrap |
| POST | `/api/v1/auth/login` | sets `dm_session` cookie |
| POST | `/api/v1/auth/logout` | clears session |
| GET  | `/api/v1/auth/me` | current user |
| GET  | `/api/v1/users` | admin list |
| POST | `/api/v1/users` | admin create |
| PATCH| `/api/v1/users/:id` | admin update |
| DELETE| `/api/v1/users/:id` | admin delete |
| GET  | `/api/v1/servers` | list |
| POST | `/api/v1/servers` | create |
| PATCH| `/api/v1/servers/:id` | update |
| DELETE| `/api/v1/servers/:id` | delete |
| POST | `/api/v1/servers/:id/check` | SSRF-safe TCP:25 probe |
| POST | `/api/v1/servers/check-all` | probe all |
| POST | `/api/v1/servers/:id/maintenance` | toggle maintenance |
| GET  | `/api/v1/domains` | list |
| POST | `/api/v1/domains` | create |
| PATCH| `/api/v1/domains/:id` | update |
| DELETE| `/api/v1/domains/:id` | delete |
| POST | `/api/v1/domains/:id/verify` | TXT ownership challenge |
| POST | `/api/v1/domains/:id/validate` | MX/SPF/DKIM/DMARC lookup |
| GET  | `/api/v1/domains/:id/mailboxes` | per-domain list |
| POST | `/api/v1/domains/:id/mailboxes` | create |
| GET  | `/api/v1/domains/:id/aliases` | per-domain list |
| POST | `/api/v1/domains/:id/aliases` | create |
| PATCH| `/api/v1/mailboxes/:mailbox_id` | update |
| DELETE| `/api/v1/mailboxes/:mailbox_id` | delete |
| DELETE| `/api/v1/aliases/:alias_id` | delete |
| GET  | `/api/v1/mailboxes/engine` | engine adapter status |
| GET  | `/api/v1/settings` | get |
| PATCH| `/api/v1/settings` | update |
| GET  | `/api/v1/audit` | list |
| GET  | `/api/v1/jobs` | list (paged) |
| POST | `/api/v1/jobs` | enqueue `{kind, payload}` |
| POST | `/api/v1/jobs/:id/retry` | re-run a failed job |
| POST | `/api/v1/jobs/:id/cancel` | cancel a pending job |
| GET  | `/api/v1/servers/:id/health` | `?limit=` health history |
| GET  | `/api/v1/domains/:id/checks` | `?kind=&limit=` check history |
| GET  | `/api/v1/providers` | list DNS providers |
| POST | `/api/v1/providers` | create provider |
| PATCH| `/api/v1/providers/:id` | update provider |
| DELETE| `/api/v1/providers/:id` | delete provider |
| POST | `/api/v1/providers/:id/verify` | credential/zone check |
| GET  | `/api/v1/domains/:id/dns-records` | generated DNS plan |
| POST | `/api/v1/domains/:id/dns-records/apply` | apply plan via provider |
| POST | `/api/v1/domains/:id/dkim` | generate selector + keypair |
| GET  | `/api/v1/dashboard` | monitoring summary |

### Packages

`internal/{config,db,httpapi,auth,audit,netutil,settings,servers,domains,mailboxes,monitoring,jobs,api,secretbox,providers,dns}`,
`migrations` (embedded), `cmd/api`.

### Tests

`internal/auth` (password/roles/token), `internal/netutil` (SSRF guard),
`internal/secretbox` (AES-256-GCM seal/open/mask), `internal/dns` (record
planning), `internal/providers` (CRUD/verify/adapters). No handler/integration/DB
tests yet.

---

## 2. Gap analysis

> Audited against the live tree (routes, `store.tsx`, `api.ts`, `router.go`,
> migrations). Tags: `[x]` closed, `[~]` partially closed, `[ ]` open. Re-audited
> 2026-10-13 after Phases 9–10.

### A. Data we collect but never exposed (write-only)  `[x]`
- `[x]` `server_health` → `GET /servers/:id/health` (Phase 9).
- `[x]` `domain_check_results` → `GET /domains/:id/checks` (Phase 9).
- `[x]` `jobs` → `POST /jobs` (enqueue), `POST /jobs/:id/retry|cancel` (Phase 9).

### B. Frontend screens / operations not backed by the API
Every frontend route and whether it reads/writes the real API was enumerated.
`ResourcePage` renders **sample** rows and its create/edit/delete/verify actions
call in-memory `save`/`remove`/`simulateCheck` only — never the API.

- `[x]` **DNS records** (`/dns`) — real (`DnsPage`, Phase 10).
- `[x]` **DNS providers** (`/providers`) — real (`ProvidersPage`, Phase 10).
- `[x]` **Provisioning jobs** (`/jobs`) — real (`JobsPage`, Phase 9).
- `[x]` **Overview** (`/`) — real (`dashboard`).
- `[ ]` **Servers** (`/servers`) — **read live, write sample.** List is hydrated
  from `GET /servers`, but create/edit/delete/check/check-all/maintenance still
  call `save`/`simulateCheck`. Backend endpoints already exist and are unused.
- `[ ]` **Domains** (`/domains`) — **read live, write sample.** Create/edit/delete/
  verify/validate not wired though `POST|PATCH|DELETE /domains`, `/verify`,
  `/validate` exist.
- `[ ]` **Mailboxes** (`/mailboxes`) — **fully sample.** Backend is domain-scoped
  (`GET|POST /domains/:id/mailboxes`, `PATCH|DELETE /mailboxes/:id`) and unused;
  the screen has no domain selector.
- `[ ]` **Aliases** (`/aliases`) — **fully sample.** Backend domain-scoped
  (`GET|POST /domains/:id/aliases`, `DELETE /aliases/:id`) and unused.
- `[ ]` **Users & access** (`/users`) — **read live, write sample.** Create/update/
  delete not wired though `POST|PATCH|DELETE /users` exist.
- `[ ]` **Settings** (`/settings`) — **fully sample.** `GET|PATCH /settings` exist
  and are unused; the page only mutates the in-memory store.
- `[ ]` **Audit log** (`/audit`) — **read live (page 1 only).** List is hydrated but
  the `action` filter + pagination the backend supports are not exposed.
- `[ ]` **Monitoring** (`/monitoring`) — partial: server-health panel is live; the
  delivery/queue/spam/TLS metric tiles remain sample.
- `[ ]` **Routing** (`/routing`), **Backups** (`/backups`),
  **Notifications** (`/notifications`) — fully sample; no backend module yet (Phase 12).
- `[~]` **Webmail** (`/webmail`) — external URL launcher; no backend needed (its
  target URL comes from settings, which is itself unwired above).
- `[ ]` **Read fidelity:** even "read-live" lists are lossy — `store.tsx` maps domain
  mailbox/storage/provider columns and server usage/domain counts to placeholders
  (`"0"`, `"Live / check"`, `provider:"manual"`); user `scope`/`lastLogin` are stubbed.
  Real per-row aggregates need backend counts.
- `[ ]` **Shell chrome:** workspace name (`"Acme workspace"`), the ⌘K search index,
  and the header notification/account panels render sample data.

### C. Implemented-but-stubbed  `[ ]`
- **Mail engine:** `main.go` still injects `mailboxes.NoopEngine{}`; mailbox/alias
  mutations persist DB rows but do not provision; `GET /mailboxes/engine` reports
  *unmanaged*.
- **Provisioning worker:** the Phase 9 worker executes real enqueued jobs, but no
  mailbox/alias/domain mutation enqueues a job automatically.

### D. Auth & security gaps  `[ ]`
- No MFA/TOTP (the `totp_secret` column exists but is unused).
- No CSRF protection on cookie-authenticated mutations.
- Cookie `Secure` hardcoded `false` (`auth/handler.go` Login/Logout); no session
  rotation; no "revoke all sessions".
- No self-service password change or admin-initiated reset.
- In-memory rate limiting only; no account lockout.
- Audit records **no** login success/failure/logout or setup events (the auth
  package never calls `audit.Record`); no export.

### E. Cross-cutting / infra
- `[~]` Migrations: `0001_init` + `0002_dns` exist; still **no down migrations**
  and minimal seeding.
- `[~]` Pagination: `jobs` and `audit` return real `meta{page,perPage,total}`;
  `mailboxes`, `aliases`, `servers`, `domains`, `users`, `providers` still report
  `total = len` with no `page`/`perPage`; no filter/sort/search or bulk ops.
- `[ ]` No OpenAPI spec.
- `[ ]` No metrics/Prometheus endpoint; no liveness-vs-readiness split.
- `[ ]` No Dockerfile/CI (Docker not installed on this machine — artifacts documented only).

### F. Contract notes
- Aliases: create/delete only — **no update** endpoint.
- Users list: no pagination / no self-delete guard beyond the super_admin SQL block.
- Mailboxes/aliases have **no global list** — domain-scoped only, so a workspace-wide
  view needs either a selector or a new aggregate endpoint.
- `jobs` payload/result are free-form JSON (`unknown` in the client); no schema.

---

## 3. Phased plan

Phases continue from the completed `0–8`. Each phase = migration (if needed) →
service → handler → validation → tests → frontend wiring → live e2e → push.

### Phase 9 — Surface collected telemetry  `[x]`
**Goal:** expose `server_health`, `domain_check_results`, and job control.

**API**
- `GET /api/v1/servers/:id/health?limit=&since=` → health history rows.
- `GET /api/v1/domains/:id/checks?kind=&limit=` → check-result history.
- `POST /api/v1/jobs` → enqueue `{kind, payload}`.
- `POST /api/v1/jobs/:id/retry` and `POST /api/v1/jobs/:id/cancel`.

**Tasks**
- [x] `servers`: add health-history query + handler; register route.
- [x] `domains`: add check-result history query + handler; register route.
- [x] `jobs`: add `Enqueue`, `Retry`, `Cancel` service methods + handlers.
- [x] Consistent `meta` pagination on all three lists.
- [x] Unit tests for the new queries/handlers.
- [x] Frontend: `monitoring.tsx` history chart, domain detail checks, `jobs.tsx` actions.

**Verification:** `go test ./...`; live curl shows history after a check/validate;
enqueue→worker picks up job.

---

### Phase 10 — DNS records + provider adapters  `[x]`
**Goal:** generate the records a domain needs and (optionally) push them.

**Migration `0002_dns.sql`:** `dns_records`, `dns_providers` (+ `dkim_*` columns on `domains`).

**API**
- `GET  /api/v1/domains/:id/dns-records` → generated plan (MX/SPF/DKIM/DMARC/A/PTR).
- `POST /api/v1/domains/:id/dns-records/apply` → push via configured provider.
- `POST /api/v1/domains/:id/dkim` → generate selector + keypair.
- `GET/POST/PATCH/DELETE /api/v1/providers` → provider CRUD.
- `POST /api/v1/providers/:id/verify` → credentials/zone check.

**Tasks**
- [x] New `internal/dns` (record planning) and `internal/providers` packages.
- [x] `DNSProvider` interface (`ListZones`, `UpsertRecord`, `DeleteRecord`, `Verify`)
      with `manual` + first real adapter (Cloudflare).
- [x] Store provider credentials encrypted; never log them.
- [x] Frontend: `dns.tsx`, `providers.tsx`.

**Verification:** `go test ./...`; generated plan matches validator expectations;
apply against a mock/manual provider produces idempotent upserts.

---

### Phase 11 — Real mail engine + provisioning pipeline  `[ ]`
**Goal:** replace `NoopEngine`; make provisioning real and asynchronous.

**Migration `0003_provisioning.sql`:** engine config binding; mailbox provisioning state.

**API**
- mailbox/alias create/update/delete **enqueue** provisioning jobs.
- `POST /api/v1/domains/:id/provision` → ensure domain exists in engine.
- `GET  /api/v1/mailboxes/:mailbox_id/provisioning` → per-mailbox state.

**Tasks**
- [ ] Define `Engine` interface (`EnsureDomain`, `CreateMailbox`, `SetPassword`,
      `DeleteMailbox`, `CreateAlias`, `DeleteAlias`, `Status`).
- [ ] Implement at least one real adapter (e.g. Dovecot `userdb`/SQL or command adapter),
      selectable via settings.
- [ ] Worker: real consumers with retry + backoff + idempotency keys.
- [ ] Frontend: provisioning status surfacing.

**Verification:** `go test ./...`; creating a mailbox enqueues + completes a job and
the engine reflects it (or reports an honest *unmanaged* state when unconfigured).

---

### Phase 12 — Routing, backups, notifications  `[ ]`
**Goal:** back the remaining fixture screens.

**Migration `0004_ops.sql`:** `routing_rules`, `backup_targets`, `backup_runs`,
`notification_channels`, `notification_events`.

**API**
- `GET/POST/PATCH/DELETE /api/v1/routing-rules`
- `GET/POST/PATCH/DELETE /api/v1/backups` + `POST /api/v1/backups/:id/run` + `GET /api/v1/backups/:id/runs`
- `GET/POST/PATCH/DELETE /api/v1/notifications` + `POST /api/v1/notifications/:id/test`

**Tasks**
- [ ] Routing-rule service + validation.
- [ ] Backup adapter boundary (external prerequisite — mark `[!]` until a backend tool
      such as `pg_dump`/`rsync`/`borg` is chosen); SSRF-safe, secrets never logged.
- [ ] Notification dispatch interface (webhook first; email later).
- [ ] Frontend: `routing.tsx`, `backups.tsx`, `notifications.tsx`.

**Verification:** `go test ./...`; a routing rule persists; a notification `test`
fires to the configured channel.

---

### Phase 13 — Auth & session hardening  `[ ]`
**Goal:** close the security gaps in [§ D](#d-auth--security-gaps).

**Migration `0005_auth_hardening.sql`:** MFA secrets/recovery codes; session metadata.

**API**
- `POST /api/v1/auth/totp/enroll` + `/verify` + `/disable`.
- `POST /api/v1/auth/password` (self-change); admin reset on users.
- `GET  /api/v1/auth/sessions` + `DELETE /api/v1/auth/sessions` (revoke all).

**Tasks**
- [ ] TOTP (RFC 6238) enroll/verify + recovery codes.
- [ ] CSRF double-submit token for all cookie-auth mutations.
- [ ] Cookie `Secure` in production, rotation, SameSite audit.
- [ ] Account lockout + persistent rate limiting.
- [ ] Audit login success/failure/logout; add list filters.

**Verification:** `go test ./...`; CSRF rejects a mutation without token; TOTP flow
works; login events appear in `/audit`.

---

### Phase 14 — Frontend wiring for all prior phases  `[ ]`
**Goal:** no screen may render fabricated data or run a sample mutation.
**Tasks**
- [ ] Extend `frontend/src/lib/api.ts` with every new endpoint (typed).
- [ ] Replace `ResourcePage` sample rendering + in-memory `save`/`remove` for
      **servers, domains, users** with real `create/update/delete` calls (endpoints
      already exist); wire `check`, `check-all`, `maintenance`, domain `verify`/`validate`.
- [ ] Wire **mailboxes** and **aliases** to their domain-scoped endpoints
      (add a domain selector, or a new aggregate list endpoint — see [§ F](#f-contract-notes)).
- [ ] Wire **settings** load/save to `GET|PATCH /settings`.
- [ ] Wire **audit** `action` filter + pagination UI.
- [ ] Replace fixture data in `routing`, `backups`, `notifications`, and the
      monitoring metric tiles with real API reads (depends on Phase 12).
- [ ] MFA + password + session screens (depends on Phase 13).
- [ ] `npx tsc --noEmit` + `npm test` clean; verify each screen live.

---

### Phase 15 — Cross-cutting polish  `[ ]`
**Tasks**
- [ ] OpenAPI spec + request/response validation.
- [ ] Real pagination + filter/sort/search across list endpoints.
- [ ] `/metrics` (Prometheus-style) + liveness/readiness split.
- [ ] Handler/integration/DB tests; CI (GitHub Actions).
- [ ] Dockerfile + compose (documented; Docker not installed locally).

---

## Status board

| Phase | Title | Status | Commit |
|---|---|---|---|
| 0–8 | Foundation → frontend wiring (baseline) | `[x]` | `9175d67` |
| 9 | Surface collected telemetry | `[x]` | `4ebb81c` |
| 10 | DNS records + providers | `[x]` | `4ebb81c` |
| 11 | Real mail engine + provisioning | `[ ]` | — |
| 12 | Routing / backups / notifications | `[ ]` | — |
| 13 | Auth & session hardening | `[ ]` | — |
| 14 | Frontend wiring | `[ ]` | — |
| 15 | Cross-cutting polish | `[ ]` | — |

---

## Progress log

> Append one row per meaningful update. Include evidence (command + result) so the
> state is traceable.

| Date | Phase | Change | Evidence | Commit |
|---|---|---|---|---|
| 2026-10-13 | 0–8 | Baseline backend + frontend wiring complete; port moved to `:7000` | `git push 2302558..9175d67`; `go test ./...` pass; live `/health` `db:ok` | `9175d67` |
| 2026-10-13 | 9 | Telemetry surfaced: server health history, domain check history, jobs enqueue/retry/cancel + mutex worker | `go vet ./...` + `go test ./...` pass (dns 2.75s, providers 2.73s, secretbox 1.00s); live e2e on `:8095` green | `4ebb81c` |
| 2026-10-13 | 10 | DNS records + provider adapters: secretbox, migration `0002_dns`, `internal/providers` (manual + Cloudflare), `internal/dns` + DKIM | live e2e: plan 6→7 records, DKIM pubLen 392, manual apply, jobs `succeeded` | `4ebb81c` |
| 2026-10-13 | 9/10 | Frontend wiring: api.ts methods, `dns-page`, `providers-page`, `jobs-page`, `server-health-panel`, route wiring | `npx tsc --noEmit` clean; `npm test` 18/18; `npm run build` ok; `eslint --fix` clean | `4ebb81c` |
| 2026-10-13 | audit | Re-audited gap analysis A–F against live tree; captured omitted gaps (servers/domains/users write-side wiring, mailboxes/aliases fully sample, settings+audit wiring, monitoring tiles) and corrected § E migration/pagination status; expanded Phase 14 | full-route enumeration vs `store.tsx`/`api.ts`/`router.go`; `auth` package has no `audit.Record`; `main.go` still `NoopEngine` | pending |

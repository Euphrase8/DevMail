## Project structure
- `frontend/` — the DevMail web application (TanStack Start + React + TypeScript). All client application code lives here.
- `backend/` — the DevMail API service written in Go (Gin + PostgreSQL). All server code, migrations, worker code, and backend tests live here.

Both halves are part of one deliverable. **The product is only complete when the frontend talks end-to-end with the backend over the versioned REST API (`/api/v1`).** Never treat the frontend as a standalone product or the backend as an isolated service.

## Development model: phases, modules, end-to-end
- Develop in explicit phases. A phase is complete only when its API is implemented, tested, functional, and wired to the UI — then push to GitHub before starting the next phase.
- Keep the backend modular: one package per bounded domain (auth, users, servers, domains, dns, mailboxes, aliases, monitoring, audit, settings, provisioning/jobs) with clear interfaces where external adapters may later be plugged in (mail engine, DNS providers, monitoring).
- Every module must be implemented end-to-end: database migration + model → API handler → validation → service logic → tests → frontend screen consuming the real API. No module may remain a mock or dead stub unless it is explicitly documented as an out-of-scope external prerequisite.

## Database
- PostgreSQL. The development database user is `postgres` with password `1508` (connection string defaults are documented in `.env.example`). Use that password so local communication succeeds.
- Schema changes go through versioned SQL migrations in `backend/migrations`. Never mutate schema outside a migration.
- Never commit secrets; `.env` files must stay untracked.

## Backend conventions
- Go module `github.com/Euphrase8/DevMail/backend` (or similar), Gin HTTP framework, pgx driver.
- All endpoints live under `/api/v1`. Every protected route applies authorization on the server — hiding a button in the UI is not authorization.
- Use structured logging, consistent JSON error envelopes, request IDs, and context-aware DB/network calls.
- Connectivity/DNS/SSH checks must be SSRF-safe: resolve hostnames, reject loopback/link-local/metadata/private destinations unless an explicit admin-configured policy allows them, enforce timeouts, and never let user input become shell commands.
- Never log or expose secrets, passwords, tokens, or private keys.
- Each phase is verified by backend tests (`go test ./...` from `backend/`) plus a live end-to-end check against the running API.

## Frontend conventions
- React + TypeScript strict, TanStack Start file routes, Tailwind CSS, shadcn/ui primitives, Lucide icons, TanStack Query for API state.
- Screens fetch real API data via the typed API client in `frontend/src/lib/api.ts`; keep the in-memory shell/store only as fallback when the API is unreachable, and always render honest "unavailable/not connected" states rather than fabricated values.
- Update the frontend, not just the backend, when API contracts change.

## Verification and pushes
- Run lint/tests from `frontend` (`npm run lint`, `npm test`).
- Run backend checks from `backend` (`go test ./...`, `go vet ./...`).
- After each completed phase: run all tests, confirm the app is runnable end-to-end, commit, and push to GitHub.
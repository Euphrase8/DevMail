## Project structure
- `frontend/` — the DevMail web application (TanStack Start + React). All application code lives here.
- `backend/` — reserved for a future service. It is intentionally empty; do not add code, config or dependencies there unless explicitly asked.

## Frontend architecture
- Keep the shared mail workspace shell and in-memory sample state in `frontend/src/components/mail`; route leaves own metadata and screen selection so backend integration can replace sample state without redesigning screens.
- Define mail resource schemas and sample fixtures in `frontend/src/lib/mail/data.ts`; management screens share form/table primitives for consistent validation and lifecycle behavior.
- This delivery is frontend-only: never imply sample statuses are verified infrastructure, persist credentials, or execute provisioning from the browser.
- Use TanStack file routes for every navigation destination; do not modify the generated route tree.
- Run lint/tests from the `frontend` directory (`npm run lint`, `npm test`).

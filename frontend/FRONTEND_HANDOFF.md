# DevMail frontend handoff

The frontend is a connected TanStack Start application. It uses the typed client
in `src/lib/api.ts` to call the Go service under `/api/v1`, carrying the
cookie-authenticated session and signed CSRF token for mutations.

Every product route reads or writes live API resources. When the API cannot be
reached, the interface reports that state instead of showing placeholder
resources or operational metrics. Mail delivery, DNS credentials and backup
destinations remain deployment configuration external to the browser.

Run `npm test`, `npm run lint`, `npx tsc --noEmit` and `npm run build`
before shipping frontend changes. Run the backend suite and live verification
script with PostgreSQL for end-to-end validation.

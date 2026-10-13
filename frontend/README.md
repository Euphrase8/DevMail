# DevMail

DevMail is a self-hosted mail infrastructure control panel. This directory contains the frontend application only — a TanStack Start / React workspace for managing mail servers, domains, DNS records, mailboxes, aliases, routing, webmail, monitoring, provisioning jobs, backups, audit logs, users and settings.

## Development

You need Node.js and npm.

```sh
npm install
npm run dev
```

The dev server starts on http://localhost:8080.

## Scripts

- `npm run dev` — start the Vite dev server
- `npm run build` — production build
- `npm run preview` — preview a production build
- `npm run lint` — run ESLint
- `npm run format` — format with Prettier
- `npm test` — run the Vitest suite

## Notes

This delivery is frontend-only. All resources, statuses and metrics are illustrative fixtures held in memory for the current browser session. No mail server, DNS provider, database, authentication service or provisioning worker is connected. See `FRONTEND_HANDOFF.md` for scope and integration boundaries.

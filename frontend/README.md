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

The frontend is part of the DevMail application and communicates with the Go API
under `/api/v1`. Set `VITE_API_URL` when the API is hosted separately; during
development the Vite proxy forwards `/api` to `http://localhost:7000`.

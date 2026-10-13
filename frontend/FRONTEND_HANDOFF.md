# DevMail frontend handoff

## Scope
This is a frontend-only delivery. All initial resources and operational metrics are illustrative fixtures. The header and resource footers identify the demo environment. No mail server, DNS provider, database, authentication service, or provisioning worker is connected.

## Screens
The overview is at `/`. Dedicated routes cover servers, domains, DNS records, mailboxes, aliases, routing, webmail, monitoring, provisioning jobs, backups, audit log, users, settings, and sign-in/admin-bootstrap/MFA previews.

## Interactive behavior
Resource pages include search, status filtering, CSV export, detail inspection, copying fields, add/edit forms, duplicate detection, validation, and confirmed sample removal. Server registration validates hostname and IPv4. Mailboxes validate password length and quota; generated passwords are not retained in fixtures. Sample settings and resources last only for the current browser session and reset on reload. No browser storage is used.

## Integration boundaries
- Replace `MailProvider` sample data and mutations with authenticated TanStack Query/API operations.
- Implement backend authorization independently of frontend controls. Apply role and resource scope on every protected operation.
- Implement domain ownership challenge issuance and verification server-side. DNS readiness, mail-engine provisioning and external delivery verification are distinct lifecycle stages.
- Connect server enrollment with restricted credentials, preflight checks, explicit privileged approval and auditable jobs. Do not accept private keys in the frontend.
- Replace the sample charts and health indicators with real timestamped monitoring data. Add polling or subscriptions only after services exist.
- Wire job retry, backup restore, password reset, user invitations, MFA, session revocation and alert acknowledgement to secured endpoints. Current controls explicitly report that no live action occurred.
- Use secret storage for mail-engine and DNS-provider credentials; disabled credential inputs intentionally avoid exposing or retaining secrets.
- The sign-in page does not authenticate or create sessions. Webmail opens only a user-configured HTTPS endpoint and does not provide automatic mailbox access.

## Design
Semantic OKLCH tokens, typography, motion, responsive layout rules, lines and visualization styling live in `src/styles.css`. Components use existing Button and Radix Dialog controls. Reduced motion is respected. No decorative cards or uploaded reference screenshots are used.

## Validation
Routing tests cover every leaf. Browser checks should include search, add/edit/remove, invalid server input, mailbox creation, filters, settings changes, mobile navigation and reduced-motion rendering. Actual infrastructure and authenticated workflows cannot be verified until backend integration exists.
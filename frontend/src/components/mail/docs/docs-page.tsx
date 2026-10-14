import {
  BookOpen,
  Database,
  Download,
  ExternalLink,
  FileKey2,
  Globe,
  LifeBuoy,
  ListChecks,
  LockKeyhole,
  Network,
  Rocket,
  Server,
  ShieldCheck,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { useMail } from "../store";

const sections: {
  id: string;
  icon: LucideIcon;
  title: string;
  intro: string;
  body: string[];
  steps?: { step: string; detail: string }[];
  links: { label: string; href: string }[];
}[] = [
  {
    id: "getting-started",
    icon: Rocket,
    title: "Create mail for your domain",
    intro: "From a bare domain to a working mailbox, in order.",
    steps: [
      {
        step: "Register your mail server",
        detail:
          "Servers → Register server. Enter the mail hostname (mail.example.com), the server's public IPv4 address, and its location. DevMail records the host for monitoring and later provisioning.",
      },
      {
        step: "Point DNS at the server",
        detail:
          "Create an A record mail.example.com → your server's IPv4. Ask your hosting provider to set the PTR (reverse DNS) for that IP to mail.example.com — only the provider can do this.",
      },
      {
        step: "Add your domain",
        detail:
          "Domains → Add domain. Choose the mail server that will host it and set a mailbox limit and quota.",
      },
      {
        step: "Verify ownership",
        detail:
          "Open the domain and copy the ownership challenge (_relay-verification.example.com = relay-verify=…). Add it as a TXT record at your DNS provider, then click Verify ownership.",
      },
      {
        step: "Install the mail DNS records",
        detail:
          'On the DNS records screen add: MX → mail.example.com (priority 10), SPF TXT "v=spf1 mx -all", a DKIM TXT at dkim._domainkey, and a DMARC TXT at _dmarc. Re-run checks until each row shows Verified.',
      },
      {
        step: "Create mailboxes",
        detail:
          "Mailboxes → Create mailbox. Provide the address, a display name, a password of at least 12 characters, and a quota. Provisioning runs as an audited job and reconciles with the mail engine.",
      },
      {
        step: "Add aliases",
        detail:
          "Aliases → Create alias to forward shared addresses (support@, billing@) to one or more real mailboxes.",
      },
      {
        step: "Configure your mail client",
        detail:
          "IMAP mail.example.com : 993 (TLS). SMTP mail.example.com : 587 (STARTTLS). Username is the full email address.",
      },
      {
        step: "Verify delivery",
        detail:
          "Watch Monitoring and Provisioning jobs for health, queue depth, and delivery events. Send a test message and confirm SPF, DKIM, and DMARC align.",
      },
    ],
    body: [
      "Follow the steps below in order. Each one has a matching screen in the workspace, and nothing reports success until the external system confirms it.",
    ],
    links: [
      { label: "SPF (RFC 7208)", href: "https://datatracker.ietf.org/doc/html/rfc7208" },
      { label: "DMARC (RFC 7489)", href: "https://datatracker.ietf.org/doc/html/rfc7489" },
    ],
  },
  {
    id: "overview",
    icon: BookOpen,
    title: "How the platform works",
    intro: "A control plane for your own mail infrastructure.",
    body: [
      "DevMail is a self-hosted administrator for one or more mail servers. You register servers, attach domains, verify ownership, organize DNS, and provision mailboxes through a professional dashboard.",
      "Every privileged step — enrollment, DNS changes, mailbox creation, restore — runs as an explicit, auditable job with preflight checks and confirmation. Nothing claims success until the external system confirms it.",
    ],
    links: [],
  },
  {
    id: "prerequisites",
    icon: Server,
    title: "Mail server prerequisites",
    intro: "What you need before registering a server.",
    body: [
      "A Linux host with a public IPv4 address, a dedicated mail hostname such as mail.example.com, and sufficient disk for the mail suite plus backups.",
      "Ownership of the address space for reverse DNS, and open routing for SMTP port 25 inbound and outbound (your hosting provider controls PTR).",
      "The selected mail engine is deployed on that host using its supported method. DevMail connects to its administrative API with dedicated, least-privilege credentials.",
    ],
    links: [
      { label: "Mailcow installation", href: "https://docs.mailcow.email/" },
      { label: "Finding your instance", href: "https://docs.mailcow.email/getstarted/install/" },
    ],
  },
  {
    id: "dns",
    icon: Globe,
    title: "DNS setup",
    intro: "Records, verification, and the honest separation of checks.",
    body: [
      "A domain is registered, then ownership is verified with a DNS TXT challenge. Only after verification may provisioning begin.",
      "MX, A/AAAA, SPF, DKIM and DMARC are generated from the real server and mail-engine configuration, compared with public observation, and revalidated after changes.",
      "Reverse DNS is a provider-managed requirement — this product explains what to request; it cannot fulfill it for you.",
    ],
    links: [
      { label: "SPF (RFC 7208)", href: "https://datatracker.ietf.org/doc/html/rfc7208" },
      { label: "DMARC (RFC 7489)", href: "https://datatracker.ietf.org/doc/html/rfc7489" },
      { label: "Cloudflare DNS API", href: "https://developers.cloudflare.com/dns/" },
    ],
  },
  {
    id: "mailstack",
    icon: Network,
    title: "Mail stack and integrations",
    intro: "Preferred engine and the adapter boundary.",
    body: [
      "Mailcow is the preferred initial mail engine: Postfix for SMTP, Dovecot for IMAP and mailboxes, Rspamd for filtering, ACME for TLS, and included webmail.",
      "DevMail uses each engine’s supported administrative API. Application state is reconciled against the engine so retries cannot create duplicates, and failures are never reported as success.",
    ],
    links: [
      { label: "Mailcow docs", href: "https://docs.mailcow.email/" },
      { label: "Postfix documentation", href: "https://www.postfix.org/documentation.html" },
      { label: "Dovecot documentation", href: "https://doc.dovecot.org/" },
      { label: "Rspamd documentation", href: "https://rspamd.com/doc/" },
      { label: "Roundcube", href: "https://roundcube.net/" },
    ],
  },
  {
    id: "security",
    icon: ShieldCheck,
    title: "Security model",
    intro: "Security defaults and where the boundaries sit.",
    body: [
      "Argon2id password hashing, secure sessions, optional TOTP, login rate limiting, CSRF protection, and strong security headers.",
      "Authorization is enforced in the backend for every operation, with tenant boundaries between organizations. SSRF protection blocks loopback, link-local, metadata, and unauthorized private destinations during every connectivity check.",
      "Secrets — SSH keys, provider tokens, mailbox passwords — are never exposed to the frontend, logs, or error messages.",
    ],
    links: [
      { label: "Let’s Encrypt", href: "https://letsencrypt.org/" },
      { label: "Certbot instructions", href: "https://certbot.eff.org/instructions" },
    ],
  },
  {
    id: "operations",
    icon: Database,
    title: "Operations and recovery",
    intro: "Backups, monitoring, and incident response.",
    body: [
      "Provisioning jobs, health checks, certificate expiry, queue depth, and delivery failures are recorded and surfaced on the dashboard. Notifications route to email or webhook.",
      "Backups cover the application database and mail data with restore verification. Restores are destructive and always require explicit confirmation.",
    ],
    links: [],
  },
  {
    id: "delivery",
    icon: LifeBuoy,
    title: "About mail delivery",
    intro: "What the product can and cannot guarantee.",
    body: [
      "Modern mail suites deliver only what their operators earn. Placement depends on IP reputation, domain age, SPF/DKIM/DMARC alignment, content, and sending volume. No control panel can guarantee inbox placement.",
      "Outbound bulk sending is intentionally limited. Opening an unrestricted relay would be a defect, not a feature.",
    ],
    links: [],
  },
];

const cards = [
  {
    icon: ListChecks,
    title: "Foundation documents",
    body: "Architecture decisions, the threat model, security checklist, acceptance criteria, and the implementation plan live in the repository under `docs/` alongside the full API reference.",
  },
  {
    icon: TriangleAlert,
    title: "Honest boundaries",
    body: "This console talks to the live DevMail backend: it provisions mailboxes and aliases, verifies DNS and domain ownership, and reads infrastructure state. Some capabilities — hosted webmail, provider OAuth, and outbound delivery tuning — are external prerequisites and report their status honestly rather than faking results.",
  },
  {
    icon: FileKey2,
    title: "Secrets policy",
    body: "Passwords, SSH keys, and provider tokens are entered and stored only through the backend. The browser never retains credentials, and nothing sensitive is written to logs.",
  },
  {
    icon: LockKeyhole,
    title: "Included software",
    body: "Domains, hosting, and DNS are external purchases. A mail engine like Mailcow is Apache-2.0 free software, but its host, volumes, and networking are your responsibility.",
  },
];

export function DocsPage() {
  const { notify } = useMail();

  const jump = (id: string) => (e: React.MouseEvent) => {
    e.preventDefault();
    document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  return (
    <div className="page-enter">
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-4">
        <div className="min-w-0">
          <div className="eyebrow mb-3">
            <span className="text-primary">ADMINISTRATION</span> / DOCUMENTATION
          </div>
          <h1 className="page-title">Documented. From install to daily operations.</h1>
          <p className="mt-3 text-[13px] text-muted-foreground">
            Decision notes, operational guides, and references for every boundary of the platform.
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => notify("Documentation artifacts are exported from the repository.")}
        >
          <Download />
          Export docs
        </Button>
      </div>

      <div className="mt-8 flex flex-col lg:h-[calc(100vh-200px)] lg:overflow-hidden">
        <div className="grid border-y border-border lg:h-full lg:grid-cols-[220px_minmax(0,1fr)] lg:gap-12">
          <nav
            aria-label="Documentation sections"
            className="shrink-0 py-6 lg:overflow-hidden lg:py-0 lg:pt-8"
          >
            <div className="lg:border-r lg:border-border lg:pr-12">
              {sections.map((s) => (
                <a
                  key={s.id}
                  href={`#${s.id}`}
                  onClick={jump(s.id)}
                  className="block border-b border-border py-3 pr-2 text-xs text-muted-foreground transition-colors hover:text-primary"
                >
                  {s.title}
                </a>
              ))}
            </div>
          </nav>

          <div className="no-scrollbar min-h-0 border-t border-border pt-2 lg:overflow-y-auto lg:border-t-0 lg:pt-8 lg:pr-2">
            {sections.map((s) => (
              <section key={s.id} id={s.id} className="border-b border-border py-7 first:pt-0">
                <div className="flex items-start gap-3">
                  <s.icon className="mt-0.5 size-4 text-primary" />
                  <div className="min-w-0">
                    <h2 className="text-base">{s.title}</h2>
                    <p className="mt-1 text-[11px] text-muted-foreground">{s.intro}</p>
                    {s.body.map((p) => (
                      <p key={p} className="mt-4 max-w-2xl text-xs leading-6 text-muted-foreground">
                        {p}
                      </p>
                    ))}
                    {s.steps && (
                      <ol className="mt-5 max-w-2xl border-y border-border">
                        {s.steps.map((st, i) => (
                          <li
                            key={st.step}
                            className="flex gap-4 border-b border-border py-4 last:border-b-0"
                          >
                            <span className="mt-0.5 grid size-6 shrink-0 place-items-center rounded-full border border-border bg-subtle font-mono text-[10px] text-primary">
                              {String(i + 1).padStart(2, "0")}
                            </span>
                            <span className="min-w-0">
                              <span className="block text-xs text-foreground">{st.step}</span>
                              <span className="mt-1 block text-[11px] leading-5 text-muted-foreground">
                                {st.detail}
                              </span>
                            </span>
                          </li>
                        ))}
                      </ol>
                    )}
                    {s.links.length > 0 && (
                      <div className="mt-5 flex flex-wrap gap-2">
                        {s.links.map((l) => (
                          <Button key={l.href} variant="outline" size="sm" asChild>
                            <a href={l.href} target="_blank" rel="noreferrer">
                              <ExternalLink className="size-3" />
                              {l.label}
                            </a>
                          </Button>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              </section>
            ))}

            <div className="grid gap-4 py-7 sm:grid-cols-2">
              {cards.map((c) => (
                <div
                  key={c.title}
                  className="flex items-start gap-3 rounded-xl border border-border bg-card p-5"
                >
                  <c.icon className="mt-0.5 size-4 text-primary" />
                  <div>
                    <h3 className="text-sm">{c.title}</h3>
                    <p className="mt-2 text-[11px] leading-5 text-muted-foreground">{c.body}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

import { Link } from "@tanstack/react-router";
import {
  ArrowUpRight,
  Check,
  Cloud,
  KeyRound,
  LockKeyhole,
  Server,
  TriangleAlert,
  Waypoints,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { useMail } from "../store";
import { Status } from "../shared/visualizations";

const providerIcons: Record<string, LucideIcon> = {
  Cloudflare: Cloud,
  Manual: Waypoints,
  "Amazon Route 53": Server,
};

const steps = [
  {
    title: "Authorize the provider",
    body: "A zone-scoped token is issued by the provider and stored encrypted by the backend — never in browser code.",
  },
  {
    title: "Preview the change set",
    body: "Expected records are generated from your actual server and mail-engine configuration. Nothing is written without explicit approval.",
  },
  {
    title: "Apply and revalidate",
    body: "Each change is recorded in the audit log, then DNS is re-read to confirm the published value matches what was intended.",
  },
];

export function ProvidersPage() {
  const { data, notify } = useMail();
  const providers = data["providers"] ?? [];
  const connected = providers.filter((p) => p.status === "Connected").length;
  const manual = providers.filter((p) => p.name === "Manual").length;

  const connect = (name: string) =>
    notify(
      `Provider authorization requires the secure backend service. ${name} was not connected and no credentials were stored.`,
    );

  return (
    <div className="page-enter">
      <div className="eyebrow mb-3">
        <span className="text-primary">INFRASTRUCTURE</span> / DNS PROVIDERS
      </div>
      <h1 className="page-title">Where your DNS meets your mail.</h1>
      <p className="mt-3 text-[13px] text-muted-foreground">
        Connect a provider to apply records automatically — or copy them manually for any DNS host.
      </p>

      <div className="mt-8 grid grid-cols-2 border-y border-border py-6 lg:grid-cols-4">
        {[
          ["INTEGRATIONS", String(providers.length), "API and manual providers"],
          ["AUTHORIZED", String(connected), "Provider connections live"],
          ["MANUAL MODE", String(manual), "Works with every DNS host"],
          ["RECORDS TRACKED", "8", "Generated per real configuration"],
        ].map(([l, v, d]) => (
          <div key={l} className="metric">
            <div className="eyebrow">{l}</div>
            <p className="metric-number mt-2">{v}</p>
            <p className="text-[11px] text-primary">{d}</p>
          </div>
        ))}
      </div>

      <div className="mt-10 grid gap-5 lg:grid-cols-3">
        {providers.map((p) => {
          const Icon = providerIcons[p.name] ?? Cloud;
          const isManual = p.name === "Manual";
          return (
            <section key={p.id} className="rounded-xl border border-border bg-card p-6 shadow-sm">
              <div className="flex items-start justify-between gap-3">
                <span className="grid h-10 w-10 place-items-center rounded border border-border">
                  <Icon className="size-4 text-muted-foreground" />
                </span>
                <Status value={p.status} />
              </div>
              <h2 className="mt-4 text-base">{p.name}</h2>
              <p className="mt-1 font-mono text-[10px] uppercase tracking-wide text-muted-foreground">
                {p["type"]} · {p["records"]}
              </p>
              <p className="mt-4 min-h-12 text-xs leading-6 text-muted-foreground">
                {p["capability"]} with an explicit preview and approval step before any change
                reaches your zone.
              </p>
              <div className="mt-5 flex flex-wrap gap-2">
                {isManual ? (
                  <>
                    <Button size="sm" variant="outline" asChild>
                      <Link to="/dns">
                        Copy records
                        <ArrowUpRight className="size-3" />
                      </Link>
                    </Button>
                    <Button size="sm" asChild>
                      <Link to="/docs">
                        Setup guide
                        <ArrowUpRight className="size-3" />
                      </Link>
                    </Button>
                  </>
                ) : (
                  <Button size="sm" onClick={() => connect(p.name)}>
                    <KeyRound />
                    Connect {p.name}
                  </Button>
                )}
              </div>
              {!isManual && p.status !== "Connected" && (
                <p className="mt-4 flex items-start gap-2 border-t border-border pt-3 text-[11px] leading-5 text-warning">
                  <LockKeyhole className="mt-0.5 size-3.5 shrink-0" />
                  Requires the secure backend service. Tokens are stored encrypted, never in the
                  frontend.
                </p>
              )}
            </section>
          );
        })}
      </div>

      <div className="mt-10 grid gap-8 border-y border-border py-8 lg:grid-cols-[1.2fr_1fr]">
        <section>
          <h2 className="section-heading">How record changes work</h2>
          <div className="mt-6">
            {steps.map((s, i) => (
              <div key={s.title} className="activity-row grid grid-cols-[auto_minmax(0,1fr)] gap-4">
                <span className="font-mono text-[10px] text-primary">0{i + 1}</span>
                <div>
                  <h3 className="text-[12px]">{s.title}</h3>
                  <p className="mt-1 text-[11px] leading-5 text-muted-foreground">{s.body}</p>
                </div>
              </div>
            ))}
          </div>
        </section>
        <section>
          <h2 className="section-heading">The honest defaults</h2>
          <ul className="mt-5 space-y-3 text-xs leading-6 text-muted-foreground">
            <li className="flex gap-2">
              <Check className="mt-0.5 size-3.5 text-primary" />
              Record sets are generated from your actual server and mail configuration, never
              hard-coded.
            </li>
            <li className="flex gap-2">
              <Check className="mt-0.5 size-3.5 text-primary" />
              SPF, DKIM and DMARC values come from the connected mail engine and your sending path.
            </li>
            <li className="flex gap-2">
              <Check className="mt-0.5 size-3.5 text-primary" />
              Unrelated records in the zone are never overwritten.
            </li>
            <li className="flex gap-2">
              <Check className="mt-0.5 size-3.5 text-primary" />
              PTR / reverse DNS is requested from your hosting provider, with guidance.
            </li>
          </ul>
          <p className="mt-6 flex items-start gap-2 border-t border-border pt-4 text-[11px] leading-5 text-warning">
            <TriangleAlert className="mt-0.5 size-3.5 shrink-0" />
            No provider is connected in this preview. Everything above is the target behavior of the
            continuing backend integration.
          </p>
        </section>
      </div>
    </div>
  );
}

import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowUpRight,
  Check,
  Cloud,
  KeyRound,
  LockKeyhole,
  Plus,
  RefreshCw,
  Server,
  Trash2,
  TriangleAlert,
  Waypoints,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { api, type DnsProvider } from "@/lib/api";
import { useMail } from "../store";
import { Status } from "../shared/visualizations";

const providerIcons: Record<string, LucideIcon> = {
  cloudflare: Cloud,
  manual: Waypoints,
  route53: Server,
};

const steps = [
  {
    title: "Authorize the provider",
    body: "A zone-scoped token is issued by the provider and encrypted at rest by the backend — never in browser code.",
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

function connectStatus(status: string): string {
  switch (status) {
    case "connected":
      return "Active";
    case "error":
      return "Failed";
    case "disabled":
      return "Suspended";
    default:
      return "Pending";
  }
}

export function ProvidersPage() {
  const { backend, notify } = useMail();
  const queryClient = useQueryClient();
  const [editor, setEditor] = useState<{
    name: string;
    kind: string;
    zone: string;
    token: string;
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const online = backend === "online";

  const providers = useQuery({
    queryKey: ["providers"],
    queryFn: api.listProviders,
    retry: false,
    enabled: online,
  });

  const list: DnsProvider[] = providers.data ?? [];
  const connected = list.filter((p) => p.status === "connected").length;
  const manual = list.filter((p) => p.kind === "manual").length;

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["providers"] });

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editor) return;
    setBusy(true);
    try {
      const created = await api.createProvider(editor);
      if (created.kind !== "manual" && created.hasToken) {
        try {
          await api.verifyProvider(created.id);
        } catch {
          /* verification failure is surfaced via status */
        }
      }
      notify(`${created.name} saved.`);
      setEditor(null);
      refresh();
    } catch (err) {
      notify(err instanceof Error ? err.message : "Could not save the provider.");
    } finally {
      setBusy(false);
    }
  };

  const verify = async (p: DnsProvider) => {
    try {
      await api.verifyProvider(p.id);
      notify(`${p.name} verified.`);
      refresh();
    } catch (err) {
      notify(err instanceof Error ? err.message : "Verification failed.");
    }
  };

  const remove = async (p: DnsProvider) => {
    try {
      await api.deleteProvider(p.id);
      notify(`${p.name} removed.`);
      refresh();
    } catch (err) {
      notify(err instanceof Error ? err.message : "Could not remove the provider.");
    }
  };

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
          ["INTEGRATIONS", String(list.length), "API and manual providers"],
          ["AUTHORIZED", String(connected), "Provider connections live"],
          ["MANUAL MODE", String(manual), "Works with every DNS host"],
          ["RECORDS TRACKED", String((providers.data ?? []).length), "Tracked per domain"],
        ].map(([l, v, d]) => (
          <div key={l} className="metric">
            <div className="eyebrow">{l}</div>
            <p className="metric-number mt-2">{v}</p>
            <p className="text-[11px] text-primary">{d}</p>
          </div>
        ))}
      </div>

      {online && (
        <div className="mt-6 flex justify-end">
          <Button
            size="sm"
            onClick={() => setEditor({ name: "", kind: "cloudflare", zone: "", token: "" })}
          >
            <Plus />
            Add provider
          </Button>
        </div>
      )}

      {!online && (
        <p className="mt-6 flex items-start gap-2 border-y border-border py-4 text-[11px] leading-5 text-warning">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" />
          The API is offline. Provider connections require the backend; nothing below is live.
        </p>
      )}

      <div className="mt-8 grid gap-5 lg:grid-cols-3">
        {list.map((p) => {
          const Icon = providerIcons[p.kind] ?? Cloud;
          return (
            <section key={p.id} className="rounded-xl border border-border bg-card p-6 shadow-sm">
              <div className="flex items-start justify-between gap-3">
                <span className="grid h-10 w-10 place-items-center rounded border border-border">
                  <Icon className="size-4 text-muted-foreground" />
                </span>
                <Status value={connectStatus(p.status)} />
              </div>
              <h2 className="mt-4 text-base">{p.name}</h2>
              <p className="mt-1 font-mono text-[10px] uppercase tracking-wide text-muted-foreground">
                {p.kind}
                {p.zone ? ` · ${p.zone}` : ""}
              </p>
              {p.lastVerifyError && (
                <p className="mt-3 text-[11px] leading-5 text-destructive">{p.lastVerifyError}</p>
              )}
              <div className="mt-5 flex flex-wrap gap-2">
                {p.kind !== "manual" && (
                  <Button size="sm" variant="outline" onClick={() => verify(p)}>
                    <RefreshCw />
                    Re-verify
                  </Button>
                )}
                <Button
                  size="sm"
                  variant="ghost"
                  className="text-muted-foreground"
                  onClick={() => remove(p)}
                >
                  <Trash2 />
                  Remove
                </Button>
              </div>
            </section>
          );
        })}
        {online && list.length === 0 && (
          <div className="rounded-xl border border-dashed border-border p-6 text-sm text-muted-foreground">
            No providers connected yet. Add one to apply DNS records automatically.
          </div>
        )}
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
              <LockKeyhole className="mt-0.5 size-3.5 text-primary" />
              Provider tokens are encrypted at rest and never returned to the browser.
            </li>
          </ul>
          <Button
            variant="ghost"
            asChild
            size="sm"
            className="mt-4 h-7 px-0 text-[11px] text-primary"
          >
            <Link to="/dns">
              Manage DNS records <ArrowUpRight className="size-3" />
            </Link>
          </Button>
        </section>
      </div>

      <Dialog open={!!editor} onOpenChange={(open) => !open && setEditor(null)}>
        <DialogContent>
          <DialogTitle>Add a DNS provider</DialogTitle>
          <DialogDescription>
            The credential is encrypted at rest and never returned by the API.
          </DialogDescription>
          {editor && (
            <form onSubmit={submit} className="mt-4 grid gap-4">
              <label className="text-xs">
                Name
                <input
                  className="mt-2"
                  value={editor.name}
                  onChange={(e) => setEditor({ ...editor, name: e.target.value })}
                  required
                />
              </label>
              <label className="text-xs">
                Kind
                <select
                  className="mt-2 h-9 w-full rounded border border-border bg-transparent px-2 text-xs"
                  value={editor.kind}
                  onChange={(e) => setEditor({ ...editor, kind: e.target.value })}
                >
                  <option value="cloudflare">Cloudflare</option>
                  <option value="manual">Manual</option>
                  <option value="route53">Amazon Route 53</option>
                </select>
              </label>
              {editor.kind !== "manual" && (
                <>
                  <label className="text-xs">
                    Zone (name or id)
                    <input
                      className="mt-2"
                      value={editor.zone}
                      onChange={(e) => setEditor({ ...editor, zone: e.target.value })}
                      required
                    />
                  </label>
                  <label className="text-xs">
                    API token
                    <input
                      className="mt-2"
                      type="password"
                      autoComplete="new-password"
                      value={editor.token}
                      onChange={(e) => setEditor({ ...editor, token: e.target.value })}
                      required
                    />
                  </label>
                </>
              )}
              <div className="mt-2 flex justify-end gap-3">
                <Button type="button" variant="ghost" onClick={() => setEditor(null)}>
                  Cancel
                </Button>
                <Button type="submit" disabled={busy}>
                  <KeyRound />
                  Save provider
                </Button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

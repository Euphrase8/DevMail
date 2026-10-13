import { useEffect, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Database, Globe2, HardDrive, KeyRound, Mail, Save, ShieldCheck } from "lucide-react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { SecurityPanel } from "../auth/security-panel";

type Tab =
  | "General"
  | "Mail engine"
  | "DNS providers"
  | "Security"
  | "Notifications"
  | "Storage"
  | "API & integrations";

const tabs: { name: Tab; icon: typeof Globe2 }[] = [
  { name: "General", icon: Globe2 },
  { name: "Mail engine", icon: Mail },
  { name: "DNS providers", icon: Database },
  { name: "Security", icon: ShieldCheck },
  { name: "Notifications", icon: Mail },
  { name: "Storage", icon: HardDrive },
  { name: "API & integrations", icon: KeyRound },
];

type Draft = {
  name: string;
  timezone: string;
  webmail: string;
  defaultQuotaBytes: string;
  mailEngine: "none" | "sql";
};

function asDraft(data: Record<string, unknown>): Draft {
  const workspace = data["workspace"] as { name?: string } | undefined;
  return {
    name: String(data["name"] ?? workspace?.name ?? "DevMail"),
    timezone: String(data["timezone"] ?? "UTC"),
    webmail: String(data["webmail"] ?? ""),
    defaultQuotaBytes: String(data["defaultQuotaBytes"] ?? 10737418240),
    mailEngine: data["mailEngine"] === "sql" ? "sql" : "none",
  };
}

function SettingRow({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <div className="settings-row">
      <div>
        <h3 className="text-sm font-medium">{title}</h3>
        <p>{description}</p>
      </div>
      <div>{children}</div>
    </div>
  );
}

function ResourceSummary({
  count,
  href,
  label,
  empty,
}: {
  count: number;
  href: string;
  label: string;
  empty: string;
}) {
  return (
    <div className="rounded-md border border-border p-4">
      <p className="text-2xl font-semibold">{count}</p>
      <p className="mt-1 text-sm">{count === 1 ? label.slice(0, -1) : label}</p>
      <p className="mt-1 text-xs text-muted-foreground">
        {count === 0 ? empty : "Managed through the connected API."}
      </p>
      <a
        className="mt-3 inline-block text-xs text-primary underline-offset-4 hover:underline"
        href={href}
      >
        Open {label.toLowerCase()}
      </a>
    </div>
  );
}

export function SettingsPage() {
  const client = useQueryClient();
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.getSettings, retry: false });
  const [tab, setTab] = useState<Tab>("General");
  const [draft, setDraft] = useState<Draft>(() => asDraft({}));
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (settings.data) setDraft(asDraft(settings.data));
  }, [settings.data]);

  const providers = useQuery({
    queryKey: ["providers"],
    queryFn: api.listProviders,
    enabled: tab === "DNS providers",
    retry: false,
  });
  const notifications = useQuery({
    queryKey: ["notifications"],
    queryFn: () => api.listOps("notifications"),
    enabled: tab === "Notifications",
    retry: false,
  });
  const backups = useQuery({
    queryKey: ["backups"],
    queryFn: () => api.listOps("backups"),
    enabled: tab === "Storage",
    retry: false,
  });

  const update = (key: keyof Draft, value: string) =>
    setDraft((current) => ({ ...current, [key]: value }));
  const save = async () => {
    const quota = Number(draft.defaultQuotaBytes);
    if (!Number.isSafeInteger(quota) || quota < 0) {
      setMessage("Default mailbox quota must be a non-negative whole number of bytes.");
      return;
    }
    setBusy(true);
    setMessage("");
    try {
      await api.updateSettings({ ...draft, defaultQuotaBytes: quota });
      await client.invalidateQueries({ queryKey: ["settings"] });
      setMessage("Workspace settings saved.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Settings could not be saved.");
    } finally {
      setBusy(false);
    }
  };

  if (settings.isLoading) return <p className="text-muted-foreground">Loading settings…</p>;
  if (settings.isError)
    return <p role="alert">Settings are unavailable. Reconnect to the API and try again.</p>;

  return (
    <div className="page-enter">
      <div className="eyebrow mb-3">
        <span className="text-primary">WORKSPACE</span> / SETTINGS
      </div>
      <h1 className="page-title">Settings hub</h1>
      <p className="mt-3 text-[13px] text-muted-foreground">
        Workspace configuration, connected services, and account controls.
      </p>

      <div className="mt-8 grid gap-8 lg:grid-cols-[220px_minmax(0,1fr)]">
        <nav aria-label="Settings sections" className="border-r border-border pr-6">
          {tabs.map(({ name, icon: Icon }) => (
            <button
              key={name}
              type="button"
              onClick={() => setTab(name)}
              className={`flex w-full items-center gap-2.5 border-b border-border py-3 text-left text-xs transition-colors ${tab === name ? "text-primary" : "text-muted-foreground hover:text-foreground"}`}
              aria-current={tab === name ? "page" : undefined}
            >
              <Icon className="size-4" />
              {name}
            </button>
          ))}
        </nav>

        <section>
          <h2 className="section-heading">{tab}</h2>
          {tab === "General" && (
            <>
              <SettingRow
                title="Workspace name"
                description="Shown in the application shell and administrative views."
              >
                <input
                  aria-label="Workspace name"
                  value={draft.name}
                  maxLength={100}
                  onChange={(event) => update("name", event.target.value)}
                />
              </SettingRow>
              <SettingRow
                title="Timezone"
                description="Used for audit entries, jobs, backups, and monitoring timestamps."
              >
                <input
                  aria-label="Timezone"
                  value={draft.timezone}
                  onChange={(event) => update("timezone", event.target.value)}
                  placeholder="UTC"
                />
              </SettingRow>
              <SettingRow
                title="Default mailbox quota"
                description="Applied when a new mailbox is created without an explicit quota."
              >
                <input
                  aria-label="Default mailbox quota"
                  type="number"
                  min={0}
                  value={draft.defaultQuotaBytes}
                  onChange={(event) => update("defaultQuotaBytes", event.target.value)}
                />
              </SettingRow>
            </>
          )}
          {tab === "Mail engine" && (
            <>
              <SettingRow
                title="Provisioning adapter"
                description="The selected adapter is persisted. An environment override takes effect after an API restart."
              >
                <select
                  aria-label="Mail engine"
                  value={draft.mailEngine}
                  onChange={(event) => update("mailEngine", event.target.value)}
                >
                  <option value="none">Unmanaged records</option>
                  <option value="sql">SQL virtual users</option>
                </select>
              </SettingRow>
              <SettingRow
                title="Provisioning status"
                description="Mailbox and alias changes are recorded in the durable job queue."
              >
                <a className="text-xs text-primary underline-offset-4 hover:underline" href="/jobs">
                  Open provisioning jobs
                </a>
              </SettingRow>
            </>
          )}
          {tab === "DNS providers" && (
            <div className="grid gap-4 sm:grid-cols-2">
              {providers.isError ? (
                <p role="alert">DNS providers are unavailable.</p>
              ) : (
                <ResourceSummary
                  count={providers.data?.length ?? 0}
                  label="DNS providers"
                  href="/providers"
                  empty="Connect a provider or use manual DNS."
                />
              )}
              <ResourceSummary
                count={0}
                label="DNS records"
                href="/dns"
                empty="Manage domain DNS records from the DNS screen."
              />
            </div>
          )}
          {tab === "Security" && <SecurityPanel />}
          {tab === "Notifications" && (
            <div className="grid gap-4 sm:grid-cols-2">
              {notifications.isError ? (
                <p role="alert">Notification destinations are unavailable.</p>
              ) : (
                <ResourceSummary
                  count={notifications.data?.length ?? 0}
                  label="notification destinations"
                  href="/notifications"
                  empty="Create a webhook destination to receive operational events."
                />
              )}
              <div className="rounded-md border border-border p-4 text-sm text-muted-foreground">
                Webhook endpoints are encrypted at rest and tested through the notification service.
              </div>
            </div>
          )}
          {tab === "Storage" && (
            <div className="grid gap-4 sm:grid-cols-2">
              {backups.isError ? (
                <p role="alert">Backup policies are unavailable.</p>
              ) : (
                <ResourceSummary
                  count={backups.data?.length ?? 0}
                  label="backup policies"
                  href="/backups"
                  empty="Create a backup policy to protect the database."
                />
              )}
              <div className="rounded-md border border-border p-4 text-sm text-muted-foreground">
                Backup runs include status, checksums, size, and retention information.
              </div>
            </div>
          )}
          {tab === "API & integrations" && (
            <>
              <SettingRow
                title="Webmail URL"
                description="The HTTPS endpoint opened by the Webmail screen for administrators."
              >
                <input
                  aria-label="Webmail URL"
                  type="url"
                  value={draft.webmail}
                  onChange={(event) => update("webmail", event.target.value)}
                  placeholder="https://webmail.example.com"
                />
              </SettingRow>
              <SettingRow
                title="API contract"
                description="The versioned REST contract describes routes and request schemas."
              >
                <a
                  className="text-xs text-primary underline-offset-4 hover:underline"
                  href="/api/v1/openapi.json"
                >
                  Open OpenAPI document
                </a>
              </SettingRow>
            </>
          )}

          {["General", "Mail engine", "API & integrations"].includes(tab) && (
            <div className="mt-8 flex items-center gap-4 border-t border-border pt-5">
              <Button disabled={busy} onClick={() => void save()}>
                <Save />
                Save changes
              </Button>
              {message && (
                <p role="status" className="text-sm">
                  {message}
                </p>
              )}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}

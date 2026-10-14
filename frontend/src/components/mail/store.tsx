import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { type MailRecord } from "@/lib/mail/data";
import { api, ApiError } from "@/lib/api";
import type {
  Alias,
  AuditEntry,
  DashboardSummary,
  DnsProvider,
  DnsRecord,
  Domain,
  Job,
  Mailbox,
  OpsKind,
  OpsResource,
  Server,
  User,
} from "@/lib/api";

type MailRecordSection = string;

type Store = {
  data: Record<MailRecordSection, MailRecord[]>;
  save: (section: string, item: MailRecord) => void;
  remove: (section: string, id: string) => void;
  refresh: (section: string) => void;
  notice: string;
  notify: (message: string) => void;
  settings: Record<string, string>;
  setSetting: (key: string, value: string) => void;
  backend: "online" | "offline" | "checking";
  session: User | null;
  sessionKnown: boolean;
  dashboard: DashboardSummary | null;
  signOut: () => Promise<void>;
};

const Context = createContext<Store | null>(null);

const GIB = 1024 ** 3;

function mapItemStatus(raw: string): string {
  switch (raw) {
    case "healthy":
    case "active":
    case "verified":
    case "success":
    case "completed":
    case "connected":
    case "succeeded":
      return "Active";
    case "degraded":
    case "maintenance":
    case "warning":
      return "Degraded";
    case "offline":
    case "suspended":
    case "disabled":
    case "failed":
    case "cancelled":
    case "missing":
    case "error":
      return "Suspended";
    default:
      return "Pending";
  }
}

function dnsStatus(raw: string): string {
  if (raw === "verified" || raw === "ok") return "Verified";
  if (raw === "missing" || raw === "failed") return "Missing";
  return "Pending";
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "Unlimited";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = bytes;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(v >= 10 ? 0 : 1)} ${units[i]}`;
}

function gbToBytes(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? Math.round(n * GIB) : 0;
}

function bytesToGb(value: number): string {
  return value > 0 ? String(Math.round(value / GIB)) : "";
}

function emailDomain(address: string): string {
  return address.split("@")[1] ?? "";
}

function localPart(address: string): string {
  return address.split("@")[0] ?? address;
}

const roleToApi: Record<string, string> = {
  "Super Administrator": "super_admin",
  "Server Administrator": "server_admin",
  "Domain Administrator": "domain_admin",
  "Read-only Operator": "operator",
};

const roleToDisplay: Record<string, string> = {
  super_admin: "Super Administrator",
  server_admin: "Server Administrator",
  domain_admin: "Domain Administrator",
  operator: "Read-only Operator",
};

function serverToRecord(s: Server): MailRecord {
  return {
    id: s.id,
    name: s.name,
    status: mapItemStatus(s.status),
    hostname: s.hostname,
    location: s.location,
    environment: s.environment,
    ip: s.ipv4 || s.ipv6,
    ipv6: s.ipv6,
    domains: String(s.domainCount ?? 0),
    usage: "Unavailable",
    cpu: "",
    memory: "",
    disk: "",
    maintenance: s.maintenance ? "Enabled" : "Disabled",
    lastCheck: s.lastCheckAt || "Never checked",
    checkError: s.lastCheckError || "None",
  };
}

function domainToRecord(d: Domain, servers: Server[]): MailRecord {
  const total = d.checkResults?.length ?? 0;
  const ok = d.checkResults?.filter((c) => c.ok).length ?? 0;
  return {
    id: d.id,
    name: d.name,
    status: mapItemStatus(d.status),
    server: d.serverId
      ? (servers.find((s) => s.id === d.serverId)?.name ?? "Assigned")
      : "Unassigned",
    provider: d.providerName || "Manual",
    mailboxes: String(d.mailboxCount ?? 0),
    storage: d.quotaBytes ? formatBytes(d.quotaBytes) : "Unlimited",
    dns: total ? `${ok} / ${total} verified` : "Awaiting checks",
    quota: bytesToGb(d.quotaBytes),
    serverId: d.serverId,
    catchall: d.catchAll ? "Enabled" : "Disabled",
    verificationName: d.verificationName || "",
    verificationToken: d.verificationToken || "",
    verified: d.verifiedAt ? "Verified" : "Unverified",
  };
}

function userToRecord(u: User): MailRecord {
  return {
    id: u.id,
    name: u.name || localPart(u.email) || "user",
    status: u.isActive ? "Active" : "Suspended",
    email: u.email,
    role: roleToDisplay[u.role] ?? u.role,
    scope: "All resources",
    lastLogin: u.lastLoginAt || "Never",
  };
}

function jobToRecord(j: Job): MailRecord {
  return {
    id: j.id,
    name: j.kind,
    status: mapItemStatus(j.status),
    target: "—",
    progress: j.status === "succeeded" ? "100%" : j.status === "running" ? "…" : "—",
    started: j.startedAt || j.createdAt,
    error: j.error || "—",
  };
}

function auditToRecord(a: AuditEntry): MailRecord {
  return {
    id: String(a.id),
    name: a.action,
    status: "Success",
    actor: a.actorEmail,
    target: `${a.resourceType} ${a.resourceId}`.trim(),
    started: a.createdAt,
  };
}

function mailboxToRecord(m: Mailbox, domainName: string): MailRecord {
  return {
    id: m.id,
    name: m.address,
    status: mapItemStatus(m.status),
    displayName: m.displayName || "",
    server: domainName,
    storage: m.quotaBytes ? formatBytes(m.quotaBytes) : "Unlimited",
    lastLogin: m.passwordSetAt ? "Password set" : "Never",
    quota: bytesToGb(m.quotaBytes),
    domainId: m.domainId,
  };
}

function aliasToRecord(a: Alias, domainName: string): MailRecord {
  return {
    id: a.id,
    name: a.source,
    status: a.enabled ? "Active" : "Suspended",
    destination: a.destination,
    domain: domainName,
    domainId: a.domainId,
  };
}

function dnsRecordToRecord(r: DnsRecord): MailRecord {
  return {
    id: r.id,
    name: r.name,
    status: dnsStatus(r.status),
    type: r.type,
    value: r.value,
    observed: r.detail || r.status,
    ttl: String(r.ttl),
    priority: String(r.priority),
    domainId: "",
  };
}

function providerToRecord(p: DnsProvider): MailRecord {
  const manual = p.kind === "manual";
  return {
    id: p.id,
    name: p.name,
    status: manual ? "Available" : p.status === "connected" ? "Connected" : "Not connected",
    type: manual ? "Any provider" : "API",
    zone: p.zone || "Not configured",
    records: "—",
    capability: manual ? "Copy & verify records" : "Zone-scoped DNS edits",
  };
}

function routingToRecord(o: OpsResource): MailRecord {
  return {
    id: o.id,
    name: o.name,
    status: o.enabled ? "Active" : "Suspended",
    match: o.config["match"] ?? "",
    destination: o.config["destination"] ?? "",
    priority: o.config["priority"] ?? "",
    type: o.config["type"] ?? "Forwarding",
  };
}

function backupToRecord(o: OpsResource): MailRecord {
  return {
    id: o.id,
    name: o.name,
    status: o.enabled ? "Active" : "Suspended",
    target: o.config["target"] ?? "",
    size: "—",
    schedule: o.config["schedule"] ?? "",
    started: "—",
    retention: o.config["retention"] ?? "",
    destination: o.config["destination"] ?? "",
  };
}

function upsertSection(
  current: Record<string, MailRecord[]>,
  section: string,
  items: MailRecord[],
): Record<string, MailRecord[]> {
  if (JSON.stringify(current[section] ?? []) === JSON.stringify(items)) return current;
  return { ...current, [section]: items };
}

function describeError(error: unknown): string {
  if (error instanceof ApiError) return error.message;
  return "The backend rejected the change.";
}

const opsKindBySection: Record<string, OpsKind> = {
  routing: "routing-rules",
  backups: "backups",
};

export function MailProvider({ children }: { children: ReactNode }) {
  const [data, setData] = useState<Record<string, MailRecord[]>>({});
  const [notice, notify] = useState("");
  const [settings, setSettings] = useState<Record<string, string>>({ name: "DevMail" });
  const queryClient = useQueryClient();

  const settingsBatch = useRef<Record<string, string>>({});
  const settingsTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const health = useQuery({
    queryKey: ["health"],
    queryFn: api.health,
    retry: false,
    refetchInterval: 30_000,
    staleTime: 15_000,
  });
  const backend: Store["backend"] =
    health.isPending && !health.error ? "checking" : health.error ? "offline" : "online";

  const servers = useQuery({ queryKey: ["servers"], queryFn: api.listServers, retry: false });
  const domains = useQuery({ queryKey: ["domains"], queryFn: api.listDomains, retry: false });
  const users = useQuery({ queryKey: ["users"], queryFn: api.listUsers, retry: false });
  const jobs = useQuery({ queryKey: ["jobs"], queryFn: api.listJobs, retry: false });
  const audit = useQuery({ queryKey: ["audit"], queryFn: () => api.listAudit(), retry: false });
  const providers = useQuery({ queryKey: ["providers"], queryFn: api.listProviders, retry: false });
  const routing = useQuery({
    queryKey: ["ops", "routing-rules"],
    queryFn: () => api.listOps("routing-rules"),
    retry: false,
  });
  const backups = useQuery({
    queryKey: ["ops", "backups"],
    queryFn: () => api.listOps("backups"),
    retry: false,
  });
  const dashboard = useQuery({
    queryKey: ["dashboard"],
    queryFn: api.dashboard,
    retry: false,
    refetchInterval: 60_000,
  });

  const session = useQuery({
    queryKey: ["session"],
    queryFn: async () => {
      try {
        return await api.me();
      } catch {
        return null;
      }
    },
    retry: false,
    staleTime: 60_000,
  });

  const liveSettings = useQuery({ queryKey: ["settings"], queryFn: api.getSettings, retry: false });

  const domainIds = domains.data?.map((d) => d.id).join(",") ?? "";
  const liveMailboxes = useQuery({
    queryKey: ["all-mailboxes", domainIds],
    queryFn: async () => {
      const lists = await Promise.all((domains.data ?? []).map((d) => api.listMailboxes(d.id)));
      return lists.flat();
    },
    enabled: !!domains.data,
  });
  const liveAliases = useQuery({
    queryKey: ["all-aliases", domainIds],
    queryFn: async () => {
      const lists = await Promise.all((domains.data ?? []).map((d) => api.listAliases(d.id)));
      return lists.flat();
    },
    enabled: !!domains.data,
  });
  const liveDns = useQuery({
    queryKey: ["all-dns", domainIds],
    queryFn: async () => {
      const lists = await Promise.all((domains.data ?? []).map((d) => api.listDnsRecords(d.id)));
      return lists.flat();
    },
    enabled: !!domains.data,
  });

  useEffect(() => {
    if (!liveSettings.data) return;
    const workspace = liveSettings.data["workspace"] as { name?: string } | undefined;
    setSettings(
      Object.fromEntries(
        Object.entries({
          ...liveSettings.data,
          name: liveSettings.data["name"] ?? workspace?.name ?? "DevMail",
        })
          .filter(([, value]) => typeof value === "string")
          .map(([key, value]) => [key, String(value)]),
      ),
    );
  }, [liveSettings.data]);

  const online = backend === "online";

  useEffect(() => {
    if (!online || !servers.data) return;
    setData((old) => upsertSection(old, "servers", servers.data!.map(serverToRecord)));
  }, [online, servers.data]);

  useEffect(() => {
    if (!online || !domains.data) return;
    setData((old) =>
      upsertSection(
        old,
        "domains",
        domains.data!.map((d) => domainToRecord(d, servers.data ?? [])),
      ),
    );
  }, [online, domains.data, servers.data]);

  useEffect(() => {
    if (!online || !users.data) return;
    setData((old) => upsertSection(old, "users", users.data!.map(userToRecord)));
  }, [online, users.data]);

  useEffect(() => {
    if (!online || !jobs.data) return;
    setData((old) => upsertSection(old, "jobs", jobs.data!.map(jobToRecord)));
  }, [online, jobs.data]);

  useEffect(() => {
    if (!online || !audit.data) return;
    setData((old) => upsertSection(old, "audit", audit.data!.map(auditToRecord)));
  }, [online, audit.data]);

  useEffect(() => {
    if (!online || !providers.data) return;
    setData((old) => upsertSection(old, "providers", providers.data!.map(providerToRecord)));
  }, [online, providers.data]);

  useEffect(() => {
    if (!online || !routing.data) return;
    setData((old) => upsertSection(old, "routing", routing.data!.map(routingToRecord)));
  }, [online, routing.data]);

  useEffect(() => {
    if (!online || !backups.data) return;
    setData((old) => upsertSection(old, "backups", backups.data!.map(backupToRecord)));
  }, [online, backups.data]);

  useEffect(() => {
    if (!online || !liveMailboxes.data) return;
    const nameOf = (id: string) => domains.data?.find((d) => d.id === id)?.name ?? "";
    setData((old) =>
      upsertSection(
        old,
        "mailboxes",
        liveMailboxes.data!.map((m) => mailboxToRecord(m, nameOf(m.domainId))),
      ),
    );
  }, [online, liveMailboxes.data, domains.data]);

  useEffect(() => {
    if (!online || !liveAliases.data) return;
    const nameOf = (id: string) => domains.data?.find((d) => d.id === id)?.name ?? "";
    setData((old) =>
      upsertSection(
        old,
        "aliases",
        liveAliases.data!.map((a) => aliasToRecord(a, nameOf(a.domainId))),
      ),
    );
  }, [online, liveAliases.data, domains.data]);

  useEffect(() => {
    if (!online || !liveDns.data) return;
    setData((old) => upsertSection(old, "dns", liveDns.data!.map(dnsRecordToRecord)));
  }, [online, liveDns.data]);

  const invalidate = (section: string) => {
    const keys: Record<string, string[]> = {
      servers: ["servers", "dashboard"],
      domains: ["domains", "dashboard"],
      users: ["users", "dashboard"],
      mailboxes: ["all-mailboxes", "dashboard"],
      aliases: ["all-aliases", "dashboard"],
      dns: ["all-dns", "domains"],
      providers: ["providers"],
      routing: ["ops"],
      backups: ["ops"],
    };
    for (const key of keys[section] ?? []) queryClient.invalidateQueries({ queryKey: [key] });
  };

  const persist = async (section: string, item: MailRecord, isNew: boolean) => {
    const serverIdFor = (name: unknown): string | undefined => {
      if (typeof name === "string" && name) {
        return servers.data?.find((s) => s.name === name)?.id;
      }
      return undefined;
    };
    const domainIdFor = (value: unknown, address: string): string | undefined => {
      const wanted = typeof value === "string" && value ? value : emailDomain(address);
      return domains.data?.find((d) => d.name === wanted)?.id;
    };

    switch (section) {
      case "servers":
        await (isNew
          ? api.createServer({
              name: item.name,
              hostname: item["hostname"] ?? "",
              ipv4: item["ip"] ?? "",
              ipv6: item["ipv6"] ?? "",
              location: item["location"] ?? "",
              environment: item["environment"] ?? "Production",
              notes: "",
            })
          : api.updateServer(item.id, {
              name: item.name,
              hostname: item["hostname"] ?? "",
              ipv4: item["ip"] ?? "",
              ipv6: item["ipv6"] ?? "",
              location: item["location"] ?? "",
              environment: item["environment"] ?? "Production",
              notes: "",
            }));
        return;
      case "domains": {
        const sid = item["serverId"] || serverIdFor(item["server"]);
        const payload: { serverId?: string; quotaBytes?: number; catchAll?: boolean } = {
          quotaBytes: gbToBytes(item["quota"]),
          catchAll: String(item["catchall"] ?? "").startsWith("Enabled"),
        };
        if (sid) payload.serverId = sid;
        await (isNew
          ? api.createDomain({ name: item.name, ...payload })
          : api.updateDomain(item.id, payload));
        return;
      }
      case "users": {
        const role = roleToApi[item["role"] ?? ""] ?? "operator";
        if (isNew) {
          await api.createUser({
            email: item["email"] ?? "",
            name: item.name,
            role,
            password: item["password"] ?? "",
          });
        } else {
          const payload: { name: string; role: string; password?: string; isActive?: boolean } = {
            name: item.name,
            role,
            isActive: item.status !== "Suspended",
          };
          if (item["password"]) payload.password = item["password"];
          await api.updateUser(item.id, payload);
        }
        return;
      }
      case "mailboxes": {
        const domainId = item["domainId"] || domainIdFor(undefined, item.name);
        if (!domainId) throw new ApiError(400, "domain", "No matching domain for this mailbox.");
        if (isNew) {
          const payload: {
            local: string;
            displayName?: string;
            quotaBytes?: number;
            password?: string;
          } = { local: localPart(item.name), quotaBytes: gbToBytes(item["quota"]) };
          if (item["displayName"]) payload.displayName = item["displayName"];
          if (item["password"]) payload.password = item["password"];
          await api.createMailbox(domainId, payload);
        } else {
          const payload: { displayName?: string; quotaBytes?: number; password?: string } = {
            quotaBytes: gbToBytes(item["quota"]),
          };
          if (item["displayName"]) payload.displayName = item["displayName"];
          if (item["password"]) payload.password = item["password"];
          await api.updateMailbox(item.id, payload);
        }
        return;
      }
      case "aliases": {
        const domainId = item["domainId"] || domainIdFor(item["domain"], item.name);
        if (!domainId) throw new ApiError(400, "domain", "No matching domain for this alias.");
        const payload = {
          source: item.name,
          destination: item["destination"] ?? "",
          forwarding: false,
        };
        if (!isNew) await api.deleteAlias(item.id);
        await api.createAlias(domainId, payload);
        return;
      }
      case "routing":
        await api.saveOps(
          "routing-rules",
          {
            name: item.name,
            enabled: item.status !== "Suspended",
            config: {
              match: item["match"] ?? "",
              destination: item["destination"] ?? "",
              priority: item["priority"] ?? "",
              type: item["type"] ?? "Forwarding",
            },
          },
          isNew ? undefined : item.id,
        );
        return;
      case "backups":
        await api.saveOps(
          "backups",
          {
            name: item.name,
            enabled: item.status !== "Suspended",
            config: {
              target: item["target"] ?? "",
              schedule: item["schedule"] ?? "",
              retention: item["retention"] ?? "",
              destination: item["destination"] ?? "",
            },
          },
          isNew ? undefined : item.id,
        );
        return;
      default:
        return;
    }
  };

  const save = (section: string, item: MailRecord) => {
    const exists = (data[section] ?? []).some((r) => r.id === item.id);
    const isNew = !exists;
    setData((old) => ({
      ...old,
      [section]: exists
        ? (old[section] || []).map((r) => (r.id === item.id ? item : r))
        : [item, ...(old[section] || [])],
    }));
    persist(section, item, isNew)
      .then(() => invalidate(section))
      .catch((error) => {
        notify(describeError(error));
        invalidate(section);
      });
  };

  const remove = (section: string, id: string) => {
    setData((old) => ({ ...old, [section]: (old[section] || []).filter((r) => r.id !== id) }));
    const run = async () => {
      switch (section) {
        case "servers":
          await api.deleteServer(id);
          return;
        case "domains":
          await api.deleteDomain(id);
          return;
        case "users":
          await api.deleteUser(id);
          return;
        case "mailboxes":
          await api.deleteMailbox(id);
          return;
        case "aliases":
          await api.deleteAlias(id);
          return;
        case "routing":
        case "backups":
          await api.deleteOps(opsKindBySection[section]!, id);
          return;
        default:
          return;
      }
    };
    run()
      .then(() => invalidate(section))
      .catch((error) => {
        notify(describeError(error));
        invalidate(section);
      });
  };

  const signOut = async () => {
    try {
      await api.logout();
    } catch {
      /* session may already be invalid */
    }
    queryClient.setQueryData(["session"], null);
    queryClient.invalidateQueries({ queryKey: ["session"] });
  };

  const supportedSettings = (batch: Record<string, string>): Record<string, unknown> => {
    const payload: Record<string, unknown> = {};
    if (batch["name"] && batch["name"].trim()) payload["name"] = batch["name"].trim();
    if (batch["timezone"]) payload["timezone"] = batch["timezone"];
    if (batch["webmail"] !== undefined) payload["webmail"] = batch["webmail"];
    if (batch["engine"] !== undefined)
      payload["mailEngine"] = batch["engine"] === "Mailcow" ? "sql" : "none";
    return payload;
  };

  const setSetting = (key: string, value: string) => {
    setSettings((old) => ({ ...old, [key]: value }));
    settingsBatch.current[key] = value;
    if (settingsTimer.current) clearTimeout(settingsTimer.current);
    settingsTimer.current = setTimeout(() => {
      const batch = settingsBatch.current;
      settingsBatch.current = {};
      const payload = supportedSettings(batch);
      if (Object.keys(payload).length === 0) return;
      api
        .updateSettings(payload)
        .then(() => queryClient.invalidateQueries({ queryKey: ["settings"] }))
        .catch((error) => notify(describeError(error)));
    }, 500);
  };

  const value: Store = {
    data,
    save,
    remove,
    refresh: invalidate,
    notice,
    notify,
    settings,
    setSetting,
    backend,
    session: session.data ?? null,
    sessionKnown: !session.isPending,
    dashboard: dashboard.data ?? null,
    signOut,
  };

  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useMail(): Store {
  const store = useContext(Context);
  if (!store) throw new Error("MailProvider is required");
  return store;
}

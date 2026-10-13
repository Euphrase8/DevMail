import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { initialData, type MailRecord } from "@/lib/mail/data";
import { api } from "@/lib/api";
import type { AuditEntry, DashboardSummary, Domain, Job, Server, User } from "@/lib/api";

type MailRecordSection = string;

type Store = {
  data: Record<MailRecordSection, MailRecord[]>;
  save: (section: string, item: MailRecord) => void;
  remove: (section: string, id: string) => void;
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

function mapItemStatus(raw: string): string {
  switch (raw) {
    case "healthy":
    case "active":
    case "verified":
      return "Active";
    case "degraded":
    case "maintenance":
      return "Degraded";
    case "offline":
    case "suspended":
    case "disabled":
    case "failed":
      return "Suspended";
    default:
      return "Pending";
  }
}

function serverToRecord(s: Server): MailRecord {
  return {
    id: s.id,
    name: s.name,
    status: mapItemStatus(s.status),
    hostname: s.hostname,
    location: s.location,
    environment: s.environment,
    ip: s.ipv4 || s.ipv6,
    domains: "0",
    usage: "Live / check",
  };
}

function domainToRecord(d: Domain): MailRecord {
  return {
    id: d.id,
    name: d.name,
    status: mapItemStatus(d.status),
    server: d.serverId ? "Assigned" : "Unassigned",
    provider: "manual",
    mailboxes: "0",
    storage: d.quotaBytes ? formatBytes(d.quotaBytes) : "Unlimited",
    dns: `${d.checkResults?.filter((c) => c.ok).length ?? 0} / ${d.checkResults?.length ?? 0} verified`,
    quota: String(d.quotaBytes),
    catchall: d.catchAll ? "true" : "false",
  };
}

function userToRecord(u: User): MailRecord {
  return {
    id: u.id,
    name: u.name || u.email.split("@")[0] || "user",
    status: u.isActive ? "Active" : "Suspended",
    email: u.email,
    role: u.role,
    scope: "workspace",
    lastLogin: "—",
  };
}

function jobToRecord(j: Job): MailRecord {
  return {
    id: j.id,
    name: j.kind,
    status: mapItemStatus(j.status),
    target: "—",
    progress: j.status === "succeeded" ? "100%" : j.status === "running" ? "…" : "—",
    started: j.createdAt,
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

function upsertSection(
  current: Record<string, MailRecord[]>,
  section: string,
  items: MailRecord[],
): Record<string, MailRecord[]> {
  if (JSON.stringify(current[section] ?? []) === JSON.stringify(items)) return current;
  return { ...current, [section]: items };
}

export function MailProvider({ children }: { children: ReactNode }) {
  const [data, setData] = useState<Record<string, MailRecord[]>>(initialData);
  const [notice, notify] = useState("");
  const [settings, setSettings] = useState<Record<string, string>>({
    name: "Acme workspace",
    timezone: "UTC",
    engine: "Mailcow",
    retention: "90",
    notifications: "On",
    mfa: "Off",
  });
  const queryClient = useQueryClient();

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
  const dashboard = useQuery({ queryKey: ["dashboard"], queryFn: api.dashboard, retry: false, refetchInterval: 60_000 });

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

  const online = backend === "online";

  useEffect(() => {
    if (!online || !servers.data) return;
    setData((old) => upsertSection(old, "servers", servers.data!.map(serverToRecord)));
  }, [online, servers.data]);

  useEffect(() => {
    if (!online || !domains.data) return;
    setData((old) => upsertSection(old, "domains", domains.data!.map(domainToRecord)));
  }, [online, domains.data]);

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
    if (!online || !dashboard.data) return;
    setSettings((old) => ({ ...old, name: "Managed mail workspace" }));
  }, [online, dashboard.data]);

  const save = (section: string, item: MailRecord) =>
    setData((old) => ({
      ...old,
      [section]: (old[section] || []).some((r) => r.id === item.id)
        ? (old[section] || []).map((r) => (r.id === item.id ? item : r))
        : [item, ...(old[section] || [])],
    }));

  const remove = (section: string, id: string) =>
    setData((old) => ({ ...old, [section]: (old[section] || []).filter((r) => r.id !== id) }));

  const signOut = async () => {
    try {
      await api.logout();
    } catch {
      /* session may already be invalid */
    }
    queryClient.setQueryData(["session"], null);
    queryClient.invalidateQueries({ queryKey: ["session"] });
  };

  const value: Store = {
    data,
    save,
    remove,
    notice,
    notify,
    settings,
    setSetting: (key, value) => setSettings((old) => ({ ...old, [key]: value })),
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
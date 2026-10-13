// Typed API client for the DevMail backend (/api/v1). All requests send
// credentials so the httpOnly dm_session cookie authenticates protected
// routes. When the API is unreachable the client throws ApiError, which callers
// surface as an honest "unavailable" state — never fabricated data.
const BASE = (import.meta.env?.["VITE_API_BASE"] as string | undefined) ?? "/api/v1";

async function csrfHeaders(init?: RequestInit): Promise<Record<string, string>> {
  if (!init?.method || ["GET", "HEAD"].includes(init.method)) return {};
  const response = await fetch(`${BASE}/auth/csrf`, { credentials: "include" });
  if (!response.ok)
    throw new ApiError(response.status, "csrf", "Could not establish a secure request.");
  const body = (await response.json()) as { data: { token: string } };
  return { "X-CSRF-Token": body.data.token };
}

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly fields: Record<string, unknown> | undefined;

  constructor(status: number, code: string, message: string, fields?: Record<string, unknown>) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.fields = fields;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(await csrfHeaders(init)),
      ...(init?.headers ?? {}),
    },
  });

  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    body = null;
  }

  if (!res.ok) {
    const err = (
      body as { error?: { code?: string; message?: string; fields?: Record<string, unknown> } }
    )?.error;
    throw new ApiError(
      res.status,
      err?.code ?? "internal",
      err?.message ?? `Request failed (${res.status}).`,
      err?.fields,
    );
  }

  return (body as { data: T }).data;
}

async function requestFull<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(await csrfHeaders(init)),
      ...(init?.headers ?? {}),
    },
  });
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    body = null;
  }
  if (!res.ok) {
    const err = (
      body as { error?: { code?: string; message?: string; fields?: Record<string, unknown> } }
    )?.error;
    throw new ApiError(
      res.status,
      err?.code ?? "internal",
      err?.message ?? `Request failed (${res.status}).`,
      err?.fields,
    );
  }
  return body as T;
}

export type Page<T> = { data: T[]; meta: { page: number; perPage: number; total: number } };
async function allPages<T>(path: string): Promise<T[]> {
  const records: T[] = [];
  let page = 1;
  for (;;) {
    const result = await requestFull<Page<T>>(
      `${path}${path.includes("?") ? "&" : "?"}page=${page}&perPage=200`,
    );
    records.push(...result.data);
    if (records.length >= result.meta.total || result.data.length === 0) return records;
    page++;
  }
}

export type User = {
  id: string;
  email: string;
  name: string;
  role: string;
  isActive: boolean;
  mfaEnabled?: boolean;
  lastLoginAt?: string;
  createdAt: string;
};

export type Server = {
  id: string;
  name: string;
  hostname: string;
  ipv4: string;
  ipv6: string;
  location: string;
  environment: string;
  notes: string;
  status: string;
  maintenance: boolean;
  domainCount?: number;
  lastCheckAt: string;
  lastCheckError: string;
};

export type DomainCheck = { kind: string; ok: boolean; detail: string; checkedAt: string };

export type Domain = {
  id: string;
  name: string;
  serverId: string;
  status: string;
  verificationToken: string;
  verificationName: string;
  verifiedAt: string;
  catchAll: boolean;
  mailboxCount?: number;
  aliasCount?: number;
  providerName?: string;
  quotaBytes: number;
  createdAt: string;
  checkResults?: DomainCheck[];
};

export type Mailbox = {
  id: string;
  domainId: string;
  local: string;
  address: string;
  displayName: string;
  quotaBytes: number;
  status: string;
  engineStatus: string;
  passwordSetAt: string;
  createdAt: string;
};

export type Alias = {
  id: string;
  domainId: string;
  source: string;
  destination: string;
  forwarding: boolean;
  enabled: boolean;
  createdAt: string;
};

export type DashboardSummary = {
  servers: {
    total: number;
    healthy: number;
    degraded: number;
    offline: number;
    pending: number;
    maintenance: number;
  };
  domains: {
    total: number;
    verified: number;
    active: number;
    degraded: number;
    pending: number;
    suspended: number;
  };
  mailboxes: number;
  aliases: number;
  users: number;
  recent: { id: number; actor: string; action: string; resource: string; createdAt: string }[];
};

export type AuditEntry = {
  id: number;
  actorEmail: string;
  action: string;
  resourceType: string;
  resourceId: string;
  detail: unknown;
  ip: string;
  createdAt: string;
};

export type Job = {
  id: string;
  kind: string;
  status: string;
  error: string;
  attempts: number;
  maxAttempts: number;
  payload?: unknown;
  result?: unknown;
  startedAt?: string;
  createdAt: string;
  finishedAt: string;
};

export type ServerHealthPoint = {
  status: string;
  latencyMs: number;
  addr: string;
  checkedAt: string;
};

export type DnsRecord = {
  id: string;
  type: string;
  name: string;
  value: string;
  ttl: number;
  priority: number;
  purpose: string;
  status: string;
  detail: string;
  appliedAt: string;
};

export type DnsProvider = {
  id: string;
  name: string;
  kind: string;
  zone: string;
  status: string;
  hasToken: boolean;
  lastVerifiedAt: string;
  lastVerifyError: string;
  createdAt: string;
};

export type JobResult = { type: string; name: string; status: string; detail: string };
export type OpsResource = {
  id: string;
  name: string;
  enabled: boolean;
  config: Record<string, string>;
  createdAt: string;
};
export type OpsRun = {
  id?: string;
  status: string;
  detail: string;
  bytes?: number;
  createdAt?: string;
};
export type OpsKind = "routing-rules" | "backups" | "notifications";

export const api = {
  resourcePage: (
    kind: "servers" | "domains" | "users",
    page = 1,
    q = "",
    sort = "name",
    order = "asc",
  ) =>
    requestFull<Page<Server | Domain | User>>(
      `/${kind}?${new URLSearchParams({ page: String(page), perPage: "25", q, sort, order })}`,
    ),
  listOps: (kind: OpsKind) => allPages<OpsResource>(`/${kind}`),
  saveOps: (kind: OpsKind, payload: Omit<OpsResource, "id" | "createdAt">, id?: string) =>
    request<OpsResource>(`/${kind}${id ? `/${id}` : ""}`, {
      method: id ? "PATCH" : "POST",
      body: JSON.stringify(payload),
    }),
  deleteOps: (kind: OpsKind, id: string) =>
    request<{ ok: boolean }>(`/${kind}/${id}`, { method: "DELETE" }),
  runBackup: (id: string) => request<OpsRun>(`/backups/${id}/run`, { method: "POST" }),
  backupRuns: (id: string) => request<OpsRun[]>(`/backups/${id}/runs`),
  testNotification: (id: string) =>
    request<OpsRun>(`/notifications/${id}/test`, { method: "POST" }),
  notificationEvents: (id: string) => request<OpsRun[]>(`/notifications/${id}/events`),
  // Health + auth
  health: () => request<{ status: string; db: string; time: string; version: string }>("/health"),
  authStatus: () => request<{ hasUsers: boolean }>("/auth/status"),
  setup: (payload: { email: string; name: string; password: string }) =>
    request<User>("/auth/setup", { method: "POST", body: JSON.stringify(payload) }),
  login: (payload: { email: string; password: string; code?: string }) =>
    request<User>("/auth/login", { method: "POST", body: JSON.stringify(payload) }),
  logout: () => request<{ ok: boolean }>("/auth/logout", { method: "POST" }),
  enrollTotp: (password: string) =>
    request<{ secret: string; uri: string }>("/auth/totp/enroll", {
      method: "POST",
      body: JSON.stringify({ password }),
    }),
  verifyTotp: (code: string) =>
    request<{ recoveryCodes: string[] }>("/auth/totp/verify", {
      method: "POST",
      body: JSON.stringify({ code }),
    }),
  disableTotp: (password: string, code: string) =>
    request<{ ok: boolean }>("/auth/totp/disable", {
      method: "POST",
      body: JSON.stringify({ password, code }),
    }),
  changePassword: (currentPassword: string, newPassword: string) =>
    request<{ ok: boolean }>("/auth/password", {
      method: "POST",
      body: JSON.stringify({ currentPassword, newPassword }),
    }),
  sessions: () =>
    request<
      {
        id: string;
        ip: string;
        userAgent: string;
        createdAt: string;
        expiresAt: string;
        lastSeenAt: string;
      }[]
    >("/auth/sessions"),
  revokeSessions: () => request<{ ok: boolean }>("/auth/sessions", { method: "DELETE" }),
  rotateSession: () => request<{ ok: boolean }>("/auth/rotate", { method: "POST" }),
  me: () => request<User>("/auth/me"),
  listUsers: () => allPages<User>("/users"),
  createUser: (payload: { email: string; name: string; role: string; password: string }) =>
    request<User>("/users", { method: "POST", body: JSON.stringify(payload) }),
  updateUser: (id: string, payload: { name: string; role: string; password?: string; isActive?: boolean }) =>
    request<User>(`/users/${id}`, { method: "PATCH", body: JSON.stringify(payload) }),
  deleteUser: (id: string) => request<{ ok: boolean }>(`/users/${id}`, { method: "DELETE" }),

  // Servers
  listServers: () => allPages<Server>("/servers"),
  createServer: (
    payload: Omit<Server, "id" | "status" | "maintenance" | "lastCheckAt" | "lastCheckError">,
  ) => request<Server>("/servers", { method: "POST", body: JSON.stringify(payload) }),
  updateServer: (
    id: string,
    payload: Omit<Server, "id" | "status" | "maintenance" | "lastCheckAt" | "lastCheckError">,
  ) => request<Server>(`/servers/${id}`, { method: "PATCH", body: JSON.stringify(payload) }),
  deleteServer: (id: string) => request<{ ok: boolean }>(`/servers/${id}`, { method: "DELETE" }),
  checkServer: (id: string) => request<Server>(`/servers/${id}/check`, { method: "POST" }),
  checkAllServers: () => request<Server[]>("/servers/check-all", { method: "POST" }),
  toggleServerMaintenance: (id: string, enabled: boolean) =>
    request<Server>(`/servers/${id}/maintenance`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    }),

  // Domains
  listDomains: () => allPages<Domain>("/domains"),
  createDomain: (payload: {
    name: string;
    serverId?: string;
    quotaBytes?: number;
    catchAll?: boolean;
  }) => request<Domain>("/domains", { method: "POST", body: JSON.stringify(payload) }),
  updateDomain: (
    id: string,
    payload: { serverId?: string; quotaBytes?: number; catchAll?: boolean },
  ) => request<Domain>(`/domains/${id}`, { method: "PATCH", body: JSON.stringify(payload) }),
  deleteDomain: (id: string) => request<{ ok: boolean }>(`/domains/${id}`, { method: "DELETE" }),
  verifyDomain: (id: string) => request<Domain>(`/domains/${id}/verify`, { method: "POST" }),
  validateDomain: (id: string) => request<Domain>(`/domains/${id}/validate`, { method: "POST" }),

  // Mailboxes + aliases
  engineStatus: () => request<{ configured: boolean; name: string }>("/mailboxes/engine"),
  mailboxProvisioning: (id: string) =>
    request<{ status: string; error: string; lastProvisionedAt: string | null }>(
      `/mailboxes/${id}/provisioning`,
    ),
  provisionDomain: (id: string) =>
    request<{ status: string }>(`/domains/${id}/provision`, { method: "POST" }),
  listMailboxes: (domainId: string) => allPages<Mailbox>(`/domains/${domainId}/mailboxes`),
  createMailbox: (
    domainId: string,
    payload: { local: string; displayName?: string; quotaBytes?: number; password?: string },
  ) =>
    request<Mailbox>(`/domains/${domainId}/mailboxes`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateMailbox: (
    id: string,
    payload: { displayName?: string; quotaBytes?: number; password?: string },
  ) => request<Mailbox>(`/mailboxes/${id}`, { method: "PATCH", body: JSON.stringify(payload) }),
  deleteMailbox: (id: string) => request<{ ok: boolean }>(`/mailboxes/${id}`, { method: "DELETE" }),
  listAliases: (domainId: string) => allPages<Alias>(`/domains/${domainId}/aliases`),
  createAlias: (
    domainId: string,
    payload: { source: string; destination: string; forwarding?: boolean },
  ) =>
    request<Alias>(`/domains/${domainId}/aliases`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  deleteAlias: (id: string) => request<{ ok: boolean }>(`/aliases/${id}`, { method: "DELETE" }),

  // Settings, monitoring, audit, jobs
  getSettings: () => request<Record<string, unknown>>("/settings"),
  updateSettings: (payload: Record<string, unknown>) =>
    request<Record<string, unknown>>("/settings", {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),
  dashboard: () => request<DashboardSummary>("/dashboard"),
  listAudit: (params?: { action?: string; page?: number; perPage?: number }) => {
    const q = new URLSearchParams();
    if (params?.action) q.set("action", params.action);
    if (params?.page) q.set("page", String(params.page));
    if (params?.perPage) q.set("perPage", String(params.perPage));
    const suffix = q.size > 0 ? `?${q.toString()}` : "";
    return request<AuditEntry[]>(`/audit${suffix}`);
  },
  auditPage: (action = "", page = 1, perPage = 25) =>
    requestFull<{ data: AuditEntry[]; meta: { page: number; perPage: number; total: number } }>(
      `/audit?${new URLSearchParams({ action, page: String(page), perPage: String(perPage) })}`,
    ),
  listJobs: () => request<Job[]>("/jobs"),
  enqueueJob: (payload: { kind: string; payload?: unknown }) =>
    request<Job>("/jobs", { method: "POST", body: JSON.stringify(payload) }),
  retryJob: (id: string) => request<Job>(`/jobs/${id}/retry`, { method: "POST" }),
  cancelJob: (id: string) => request<Job>(`/jobs/${id}/cancel`, { method: "POST" }),

  // Phase 9: history
  getServerHealth: (id: string, limit = 50) =>
    request<ServerHealthPoint[]>(`/servers/${id}/health?limit=${limit}`),
  getDomainChecks: (id: string, params?: { kind?: string; limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.kind) q.set("kind", params.kind);
    if (params?.limit) q.set("limit", String(params.limit));
    const suffix = q.size > 0 ? `?${q.toString()}` : "";
    return request<DomainCheck[]>(`/domains/${id}/checks${suffix}`);
  },

  // Phase 10: DNS records + providers
  listDnsRecords: (domainId: string) => request<DnsRecord[]>(`/domains/${domainId}/dns-records`),
  generateDkim: (domainId: string, selector = "default") =>
    request<{ selector: string; publicKey: string; record: DnsRecord }>(
      `/domains/${domainId}/dkim`,
      {
        method: "POST",
        body: JSON.stringify({ selector }),
      },
    ),
  applyDnsRecords: (domainId: string, providerId: string) =>
    requestFull<{ data: DnsRecord[]; results: JobResult[] }>(
      `/domains/${domainId}/dns-records/apply`,
      {
        method: "POST",
        body: JSON.stringify({ providerId }),
      },
    ),
  listProviders: () => allPages<DnsProvider>("/providers"),
  createProvider: (payload: { name: string; kind: string; zone?: string; token?: string }) =>
    request<DnsProvider>("/providers", { method: "POST", body: JSON.stringify(payload) }),
  updateProvider: (
    id: string,
    payload: { name: string; kind: string; zone?: string; token?: string },
  ) => request<DnsProvider>(`/providers/${id}`, { method: "PATCH", body: JSON.stringify(payload) }),
  deleteProvider: (id: string) =>
    request<{ ok: boolean }>(`/providers/${id}`, { method: "DELETE" }),
  verifyProvider: (id: string) =>
    request<DnsProvider>(`/providers/${id}/verify`, { method: "POST" }),
};

export default api;

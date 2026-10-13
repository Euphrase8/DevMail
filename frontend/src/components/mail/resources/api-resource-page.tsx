import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Domain, type Server, type User } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { ServerHealthPanel } from "../shared/server-health-panel";

type Kind = "servers" | "domains" | "users";
type Item = Server | Domain | User;
export function ApiResourcePage({ kind }: { kind: Kind }) {
  const client = useQueryClient();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState("name");
  const rows = useQuery({
    queryKey: [kind, "page", page, search, sort],
    queryFn: () => api.resourcePage(kind, page, search, sort),
  });
  const servers = useQuery({
    queryKey: ["servers"],
    queryFn: api.listServers,
    enabled: kind === "domains",
  });
  const [editor, setEditor] = useState<Record<string, string> | null>(null);
  const [detail, setDetail] = useState<Item | null>(null);
  const [deletion, setDeletion] = useState<Item | null>(null);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const checks = useQuery({
    queryKey: ["domain-checks", detail?.id],
    queryFn: () => api.getDomainChecks(detail!.id),
    enabled: kind === "domains" && !!detail,
  });
  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setMessage("");
    try {
      await fn();
      await client.invalidateQueries();
      setMessage("Saved to the service.");
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Operation failed.");
    } finally {
      setBusy(false);
    }
  };
  const edit = (row?: Item) => {
    setEditor(
      row
        ? Object.fromEntries(Object.entries(row).map(([key, value]) => [key, String(value)]))
        : {
            name: "",
            hostname: "",
            ipv4: "",
            ipv6: "",
            location: "",
            environment: "production",
            notes: "",
            serverId: "",
            quotaBytes: "10737418240",
            catchAll: "false",
            role: "operator",
            email: "",
            password: "",
          },
    );
  };
  const save = async () => {
    if (!editor) return;
    const id = editor["id"];
    const name = editor["name"] ?? "";
    if (kind === "servers") {
      const payload = {
        name,
        hostname: editor["hostname"] ?? "",
        ipv4: editor["ipv4"] ?? "",
        ipv6: editor["ipv6"] ?? "",
        location: editor["location"] ?? "",
        environment: editor["environment"] ?? "production",
        notes: editor["notes"] ?? "",
      };
      if (id) await api.updateServer(id, payload);
      else await api.createServer(payload);
    }
    if (kind === "domains") {
      const payload = {
        serverId: editor["serverId"] ?? "",
        quotaBytes: Number(editor["quotaBytes"] ?? 0),
        catchAll: editor["catchAll"] === "true",
      };
      if (id) await api.updateDomain(id, payload);
      else await api.createDomain({ name, ...payload });
    }
    if (kind === "users") {
      const payload = {
        name,
        email: editor["email"] ?? "",
        role: editor["role"] ?? "operator",
        password: editor["password"] ?? "",
      };
      if (id) await api.updateUser(id, payload);
      else await api.createUser(payload);
    }
    setEditor(null);
  };
  const fields =
    kind === "servers"
      ? ["name", "hostname", "ipv4", "ipv6", "location", "environment", "notes"]
      : kind === "domains"
        ? ["name", "serverId", "quotaBytes", "catchAll"]
        : ["name", "email", "role", "password"];
  const title = kind[0]!.toUpperCase() + kind.slice(1);
  const shown = rows.data?.data ?? [];
  return (
    <div className="space-y-6">
      <div className="flex justify-between">
        <h1 className="page-title">{title}</h1>
        <Button onClick={() => edit()}>Add {kind.slice(0, -1)}</Button>
      </div>
      {rows.isError && (
        <p role="alert">Service unavailable or access denied. Records could not be loaded.</p>
      )}
      {rows.isPending && <p>Loading records…</p>}
      <div className="toolbar">
        <input
          aria-label={`Search ${kind}`}
          placeholder="Search records"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(1);
          }}
        />
        {kind === "servers" && (
          <Button disabled={busy} onClick={() => void run(api.checkAllServers)}>
            Check all servers
          </Button>
        )}
        <select
          aria-label="Sort records"
          value={sort}
          onChange={(e) => {
            setSort(e.target.value);
            setPage(1);
          }}
        >
          <option value="name">Name</option>
          <option value="createdAt">Created time</option>
        </select>
        <Button variant="outline" onClick={() => void rows.refetch()}>
          Refresh
        </Button>
      </div>
      {message && <p role="status">{message}</p>}
      <table className="data-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>{kind === "users" ? "Role" : "Status"}</th>
            <th>Details</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {shown.map((row) => (
            <tr key={row.id}>
              <td>
                <button onClick={() => setDetail(row)}>{row.name}</button>
              </td>
              <td>{"role" in row ? row.role : row.status}</td>
              <td>
                {"hostname" in row
                  ? `${row.hostname} · ${row.ipv4 || row.ipv6}`
                  : "email" in row
                    ? row.email
                    : `Quota ${row.quotaBytes} bytes`}
              </td>
              <td>
                <Button variant="ghost" onClick={() => edit(row)}>
                  Edit
                </Button>
                <Button variant="ghost" onClick={() => setDeletion(row)}>
                  Delete
                </Button>
                {kind === "servers" && (
                  <>
                    <Button disabled={busy} onClick={() => void run(() => api.checkServer(row.id))}>
                      Check
                    </Button>
                    <Button
                      disabled={busy}
                      onClick={() =>
                        void run(() =>
                          api.toggleServerMaintenance(row.id, !(row as Server).maintenance),
                        )
                      }
                    >
                      {(row as Server).maintenance ? "End maintenance" : "Maintenance"}
                    </Button>
                  </>
                )}
                {kind === "domains" && (
                  <>
                    <Button
                      disabled={busy}
                      onClick={() => void run(() => api.verifyDomain(row.id))}
                    >
                      Verify ownership
                    </Button>
                    <Button
                      disabled={busy}
                      onClick={() => void run(() => api.validateDomain(row.id))}
                    >
                      Validate DNS
                    </Button>
                    <Button
                      disabled={busy}
                      onClick={() => void run(() => api.provisionDomain(row.id))}
                    >
                      Provision
                    </Button>
                  </>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {!rows.isPending && !rows.isError && !shown.length && <p>No records found.</p>}
      <div className="flex gap-4">
        <Button disabled={page === 1} onClick={() => setPage(page - 1)}>
          Previous
        </Button>
        <span>
          Page {page} � {rows.data?.meta.total ?? 0} records
        </span>
        <Button
          disabled={!rows.data || page * 25 >= rows.data.meta.total}
          onClick={() => setPage(page + 1)}
        >
          Next
        </Button>
      </div>
      <Dialog
        open={!!editor}
        onOpenChange={(open) => {
          if (!open) setEditor(null);
        }}
      >
        <DialogContent className="dialog-scroll">
          <DialogTitle>
            {editor?.["id"] ? "Edit" : "Add"} {kind.slice(0, -1)}
          </DialogTitle>
          <DialogDescription>Changes are saved to the connected service.</DialogDescription>
          {editor && (
            <form
              className="grid gap-3"
              onSubmit={(e) => {
                e.preventDefault();
                void run(save);
              }}
            >
              {fields.map((key) => (
                <label key={key}>
                  {key === "quotaBytes"
                    ? "Quota (bytes)"
                    : key === "serverId"
                      ? "Server"
                      : key === "catchAll"
                        ? "Catch all"
                        : key}
                  {key === "serverId" ? (
                    <select
                      value={editor[key] ?? ""}
                      onChange={(e) => setEditor({ ...editor, [key]: e.target.value })}
                    >
                      <option value="">Unassigned</option>
                      {(servers.data ?? []).map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.name}
                        </option>
                      ))}
                    </select>
                  ) : key === "role" || key === "catchAll" ? (
                    <select
                      value={editor[key] ?? ""}
                      onChange={(e) => setEditor({ ...editor, [key]: e.target.value })}
                    >
                      {(key === "role"
                        ? ["operator", "domain_admin", "server_admin", "super_admin"]
                        : ["false", "true"]
                      ).map((value) => (
                        <option key={value}>{value}</option>
                      ))}
                    </select>
                  ) : (
                    <input
                      value={editor[key] ?? ""}
                      disabled={
                        !!editor["id"] &&
                        (key === "email" || (kind === "domains" && key === "name"))
                      }
                      required={
                        ["name", "hostname", "email"].includes(key) ||
                        (key === "password" && !editor["id"])
                      }
                      min={0}
                      minLength={key === "password" ? 12 : undefined}
                      type={
                        key === "password"
                          ? "password"
                          : key === "quotaBytes"
                            ? "number"
                            : key === "email"
                              ? "email"
                              : "text"
                      }
                      autoComplete={key === "password" ? "new-password" : "off"}
                      onChange={(e) => setEditor({ ...editor, [key]: e.target.value })}
                    />
                  )}
                </label>
              ))}
              {message && <p role="alert">{message}</p>}
              <Button disabled={busy}>Save</Button>
            </form>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!deletion}
        onOpenChange={(open) => {
          if (!open) setDeletion(null);
        }}
      >
        <DialogContent>
          <DialogTitle>Delete {deletion?.name}?</DialogTitle>
          <DialogDescription>This changes a live workspace record.</DialogDescription>
          <Button
            variant="destructive"
            disabled={busy}
            onClick={() =>
              void run(async () => {
                if (!deletion) return;
                if (kind === "servers") await api.deleteServer(deletion.id);
                if (kind === "domains") await api.deleteDomain(deletion.id);
                if (kind === "users") await api.deleteUser(deletion.id);
                setDeletion(null);
              })
            }
          >
            Delete
          </Button>
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!detail}
        onOpenChange={(open) => {
          if (!open) setDetail(null);
        }}
      >
        <DialogContent className="dialog-scroll">
          <DialogTitle>{detail?.name}</DialogTitle>
          <DialogDescription>Live resource details</DialogDescription>
          {detail &&
            Object.entries(detail).map(([key, value]) => (
              <p className="text-xs break-all" key={key}>
                {key}: {typeof value === "object" ? JSON.stringify(value) : String(value)}
              </p>
            ))}
          {kind === "domains" && detail && "verificationToken" in detail && (
            <>
              <p>Publish TXT at {detail.verificationName}</p>
              <code>dmverify={detail.verificationToken}</code>
              {(checks.data ?? []).map((check, i) => (
                <p key={i}>
                  {check.kind}: {check.ok ? "Passed" : "Failed"} · {check.detail}
                </p>
              ))}
            </>
          )}
          {kind === "servers" && detail && <ServerHealthPanel />}
        </DialogContent>
      </Dialog>
    </div>
  );
}

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type OpsKind, type OpsResource } from "@/lib/api";
import { Button } from "@/components/ui/button";

export function OpsPage({ kind }: { kind: OpsKind }) {
  const client = useQueryClient();
  const records = useQuery({ queryKey: [kind], queryFn: () => api.listOps(kind) });
  const [id, setId] = useState<string>();
  const [name, setName] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [config, setConfig] = useState<Record<string, string>>(
    kind === "backups" ? { kind: "postgres" } : {},
  );
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [inspect, setInspect] = useState("");
  const history = useQuery({
    queryKey: [kind, inspect, "history"],
    queryFn: () => (kind === "backups" ? api.backupRuns(inspect) : api.notificationEvents(inspect)),
    enabled: !!inspect && kind !== "routing-rules",
  });
  const title =
    kind === "routing-rules" ? "Routing rules" : kind === "backups" ? "Backups" : "Notifications";
  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setMessage("");
    try {
      const result = await fn();
      if (result && typeof result === "object" && "status" in result && "detail" in result)
        setMessage(`${result.status}: ${result.detail}`);
      else setMessage("Saved.");
      await client.invalidateQueries({ queryKey: [kind] });
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Operation failed.");
    } finally {
      setBusy(false);
    }
  };
  const edit = (row: OpsResource) => {
    setId(row.id);
    setName(row.name);
    setEnabled(row.enabled);
    setConfig({ ...row.config });
  };
  return (
    <div className="space-y-6">
      <h1 className="page-title">{title}</h1>
      <p className="text-muted-foreground text-sm">
        {kind === "backups"
          ? "Database snapshots use the administrator-configured backup directory. Mail files require an external storage backup tool."
          : kind === "notifications"
            ? "Webhook destinations must be allowed by the server's network policy."
            : "Rules persist in the workspace. Apply them through your external mail server's routing configuration."}
      </p>
      {records.isError && <p role="alert">Service unavailable. Records could not be loaded.</p>}
      <form
        className="grid gap-4 max-w-xl"
        onSubmit={(e) => {
          e.preventDefault();
          void run(async () => {
            await api.saveOps(kind, { name, enabled, config }, id);
            setId(undefined);
            setName("");
            setConfig(kind === "backups" ? { kind: "postgres" } : {});
          });
        }}
      >
        <label>
          Name
          <input required value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        {(kind === "routing-rules"
          ? ["source", "destination"]
          : kind === "notifications"
            ? ["url"]
            : []
        ).map((key) => (
          <label key={key}>
            {key === "url"
              ? "Webhook URL"
              : key === "source"
                ? "Source email"
                : "Destination email"}
            <input
              required
              type={key === "url" ? "url" : "email"}
              value={config[key] ?? ""}
              onChange={(e) => setConfig({ ...config, [key]: e.target.value })}
            />
          </label>
        ))}
        <label className="flex gap-2">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Enabled
        </label>
        <div className="flex gap-2">
          <Button disabled={busy}>{id ? "Update" : "Create"}</Button>
          {id && (
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setId(undefined);
                setName("");
              }}
            >
              Cancel editing
            </Button>
          )}
        </div>
      </form>
      {message && <p role="status">{message}</p>}
      <table className="data-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>State</th>
            <th>Configuration</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {(records.data ?? []).map((row) => (
            <tr key={row.id}>
              <td>{row.name}</td>
              <td>{row.enabled ? "Enabled" : "Disabled"}</td>
              <td>
                {Object.entries(row.config).map(([key, value]) => (
                  <div key={key}>
                    {key}: {value}
                  </div>
                ))}
              </td>
              <td>
                <Button disabled={busy} variant="ghost" onClick={() => edit(row)}>
                  Edit
                </Button>
                <Button
                  disabled={busy}
                  variant="ghost"
                  onClick={() => void run(() => api.deleteOps(kind, row.id))}
                >
                  Delete
                </Button>
                {kind !== "routing-rules" && (
                  <>
                    <Button
                      disabled={busy || !row.enabled}
                      onClick={() =>
                        void run(() =>
                          kind === "backups" ? api.runBackup(row.id) : api.testNotification(row.id),
                        )
                      }
                    >
                      {kind === "backups" ? "Run backup" : "Send test"}
                    </Button>
                    <Button variant="ghost" onClick={() => setInspect(row.id)}>
                      History
                    </Button>
                  </>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {inspect && kind !== "routing-rules" && (
        <div>
          <h2>Run history</h2>
          {history.isError && <p role="alert">History unavailable.</p>}
          {(history.data ?? []).map((item, i) => (
            <p key={item.id ?? i}>
              {item.createdAt} · {item.status} · {item.detail}
              {item.bytes !== undefined ? ` · ${item.bytes} bytes` : ""}
            </p>
          ))}
        </div>
      )}
    </div>
  );
}

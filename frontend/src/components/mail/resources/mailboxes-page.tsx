import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";

export function MailboxesPage({ aliases = false }: { aliases?: boolean }) {
  const client = useQueryClient();
  const domains = useQuery({ queryKey: ["domains"], queryFn: api.listDomains });
  const [selected, setSelected] = useState("");
  const domainId = selected || domains.data?.[0]?.id || "";
  const [local, setLocal] = useState("");
  const [destination, setDestination] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [quota, setQuota] = useState("1");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const engine = useQuery({ queryKey: ["engine"], queryFn: api.engineStatus });
  const mailboxes = useQuery({
    queryKey: ["mailboxes", domainId],
    queryFn: () => api.listMailboxes(domainId),
    enabled: !!domainId && !aliases,
    refetchInterval: 2000,
  });
  const forwards = useQuery({
    queryKey: ["aliases", domainId],
    queryFn: () => api.listAliases(domainId),
    enabled: !!domainId && aliases,
  });
  const run = async (operation: () => Promise<unknown>) => {
    setBusy(true);
    setMessage("");
    try {
      await operation();
      await client.invalidateQueries();
      setMessage("Saved to the service.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Operation failed.");
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="space-y-6">
      <h1 className="page-title">{aliases ? "Aliases" : "Mailboxes"}</h1>
      <label>
        Engine selection (takes effect after API restart)
        <select
          aria-label="Engine selection"
          defaultValue=""
          onChange={(e) => void run(() => api.updateSettings({ mailEngine: e.target.value }))}
        >
          <option value="" disabled>
            Choose adapter
          </option>
          <option value="none">Unmanaged</option>
          <option value="sql">SQL virtual users</option>
        </select>
      </label>
      <p className="text-sm text-muted-foreground">
        Mail engine:{" "}
        {engine.data?.configured
          ? engine.data.name
          : "unmanaged — external mail server is not configured"}
      </p>
      {(domains.isError || mailboxes.isError || forwards.isError) && (
        <p role="alert">Service unavailable. Records could not be loaded.</p>
      )}
      <label>
        Domain
        <select aria-label="Domain" value={domainId} onChange={(e) => setSelected(e.target.value)}>
          {(domains.data ?? []).map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </select>
      </label>
      {!domainId && <p>Add a domain before creating {aliases ? "aliases" : "mailboxes"}.</p>}
      <form
        className="grid gap-4 max-w-xl"
        onSubmit={(e) => {
          e.preventDefault();
          void run(async () => {
            if (aliases)
              await api.createAlias(domainId, { source: local, destination, forwarding: true });
            else
              await api.createMailbox(domainId, {
                local,
                displayName,
                password,
                quotaBytes: Number(quota) * 1073741824,
              });
            setPassword("");
            setLocal("");
          });
        }}
      >
        <label>
          Local address
          <input
            required
            aria-label="Local address"
            value={local}
            onChange={(e) => setLocal(e.target.value)}
          />
        </label>
        {aliases ? (
          <label>
            Destination
            <input
              required
              type="email"
              value={destination}
              onChange={(e) => setDestination(e.target.value)}
            />
          </label>
        ) : (
          <>
            <label>
              Display name
              <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
            </label>
            <label>
              Password
              <input
                required
                type="password"
                minLength={12}
                autoComplete="new-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </label>
            <label>
              Quota (GB)
              <input
                type="number"
                min={0}
                required
                value={quota}
                onChange={(e) => setQuota(e.target.value)}
              />
            </label>
          </>
        )}
        <Button disabled={busy || !domainId}>Create {aliases ? "alias" : "mailbox"}</Button>
      </form>
      {message && <p role="status">{message}</p>}
      <table className="data-table">
        <thead>
          <tr>
            <th>Address</th>
            <th>{aliases ? "Destination" : "Provisioning"}</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {aliases
            ? (forwards.data ?? []).map((a) => (
                <tr key={a.id}>
                  <td>{a.source}</td>
                  <td>{a.destination}</td>
                  <td>
                    <Button disabled={busy} onClick={() => void run(() => api.deleteAlias(a.id))}>
                      Delete
                    </Button>
                  </td>
                </tr>
              ))
            : (mailboxes.data ?? []).map((m) => (
                <tr key={m.id}>
                  <td>{m.address}</td>
                  <td>{m.engineStatus}</td>
                  <td>
                    <Button
                      disabled={busy}
                      onClick={() => {
                        setLocal(m.local);
                        setDisplayName(m.displayName);
                        setQuota(String(m.quotaBytes / 1073741824));
                      }}
                    >
                      Use values
                    </Button>
                    <Button
                      disabled={busy}
                      onClick={() =>
                        void run(() =>
                          api.updateMailbox(m.id, {
                            displayName,
                            quotaBytes: Number(quota) * 1073741824,
                            ...(password ? { password } : {}),
                          }),
                        )
                      }
                    >
                      Update from form
                    </Button>
                    <Button disabled={busy} onClick={() => void run(() => api.deleteMailbox(m.id))}>
                      Delete
                    </Button>
                  </td>
                </tr>
              ))}
        </tbody>
      </table>
    </div>
  );
}

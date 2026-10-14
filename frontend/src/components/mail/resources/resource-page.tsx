import { useEffect, useState } from "react";
import { useLocation } from "@tanstack/react-router";
import {
  ArrowRight,
  Copy,
  Download,
  Eye,
  KeyRound,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { api, ApiError } from "@/lib/api";
import { modules, type MailRecord, type Section } from "@/lib/mail/data";
import { useMail } from "../store";
import { Status } from "../shared/visualizations";
export function ResourcePage({ section }: { section: Section }) {
  const { data, save, remove, refresh, notify, backend } = useMail();
  const config = modules[section]!;
  const rows = data[section] || [];
  const location = useLocation();
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("All statuses");
  const [editor, setEditor] = useState<MailRecord | null>(null);
  const [detail, setDetail] = useState<MailRecord | null>(null);
  const [deletion, setDeletion] = useState<MailRecord | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const describe = (e: unknown) =>
    e instanceof ApiError ? e.message : "The request failed. Try again.";
  const newRecord = () => {
    setError("");
    setEditor({
      id: `${section}-${Date.now()}`,
      name: "",
      status: "Pending",
      ...Object.fromEntries(
        config.fields.filter((f) => f.options).map((f) => [f.key, f.options?.[0] || ""]),
      ),
    });
  };
  useEffect(() => {
    if (location.searchStr.includes("action=new")) newRecord();
  }, [location.searchStr]);
  const shown = rows.filter(
    (r) =>
      (filter === "All statuses" || r.status === filter) &&
      Object.values(r).some((v) => v.toLowerCase().includes(query.toLowerCase())),
  );
  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!editor) return;
    if (
      section === "servers" &&
      (!/^([a-z0-9-]+\.)+[a-z]{2,}$/i.test(editor["hostname"] || "") ||
        !(editor["ip"] || "").split(".").every((v) => /^\d{1,3}$/.test(v) && Number(v) < 256) ||
        (editor["ip"] || "").split(".").length !== 4)
    ) {
      setError("Enter a valid mail hostname and IPv4 address.");
      return;
    }
    if (section === "domains" && !/^([a-z0-9-]+\.)+[a-z]{2,}$/i.test(editor.name)) {
      setError("Enter a valid domain, such as example.com.");
      return;
    }
    if (
      section === "mailboxes" &&
      (editor["password"] || "").length < 12 &&
      !rows.some((r) => r.id === editor.id)
    ) {
      setError("Use a password with at least 12 characters.");
      return;
    }
    if (
      ["mailboxes", "domains"].includes(section) &&
      editor["quota"] &&
      Number(editor["quota"]) < 1
    ) {
      setError("Quota must be at least 1 GB.");
      return;
    }
    const existing = rows.find(
      (r) => r.id !== editor.id && r.name.toLowerCase() === editor.name.toLowerCase(),
    );
    if (existing) {
      setError(`This ${config.singular} already exists.`);
      return;
    }
    const safe = { ...editor };
    delete safe["password"];
    save(section, safe);
    setEditor(null);
    notify(`${config.singular} saved.`);
  };
  const runChecks = async () => {
    setBusy(true);
    try {
      if (section === "servers") {
        await api.checkAllServers();
      } else if (section === "domains" || section === "dns") {
        for (const d of data["domains"] ?? []) await api.validateDomain(d.id);
      }
      refresh(section);
      notify("Checks submitted. Results refresh shortly.");
    } catch (e) {
      notify(describe(e));
    } finally {
      setBusy(false);
    }
  };
  const verifyOwnership = async (id: string) => {
    setBusy(true);
    try {
      await api.verifyDomain(id);
      refresh("domains");
      notify("Ownership verification submitted.");
    } catch (e) {
      notify(describe(e));
    } finally {
      setBusy(false);
    }
  };
  const exportCsv = () => {
    const csv = [
      config.columns.map((c) => c[1]),
      ...shown.map((r) => config.columns.map((c) => r[c[0]] || "")),
    ]
      .map((row) => row.map((s) => `"${s.replaceAll('"', '""')}"`).join(","))
      .join("\n");
    const url = URL.createObjectURL(new Blob([csv], { type: "text/csv" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = `devmail-${section}.csv`;
    link.click();
    URL.revokeObjectURL(url);
    notify(`Exported ${shown.length} ${config.title.toLowerCase()}.`);
  };
  const copy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      notify("Copied to clipboard.");
    } catch {
      notify("Clipboard access unavailable. Select and copy the value manually.");
    }
  };
  if (!config) return null;
  return (
    <div className="page-enter">
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4">
        <div className="min-w-0">
          <div className="eyebrow mb-3">
            <span className="text-primary">WORKSPACE</span> / {section.toUpperCase()}
          </div>
          <h1 className="page-title">{config.title}</h1>
          <p className="mt-3 text-[13px] text-muted-foreground">{config.description}</p>
        </div>
        {config.action && (
          <Button size="sm" onClick={newRecord}>
            <Plus />
            <span className="hidden sm:inline">{config.action}</span>
          </Button>
        )}
      </div>
      <div className="mt-8 flex flex-wrap gap-8 border-y border-border py-5">
        <div>
          <div className="eyebrow">TOTAL {section.toUpperCase()}</div>
          <div className="mt-2 text-2xl">{rows.length}</div>
        </div>
        <div className="border-l border-border pl-8">
          <div className="eyebrow">HEALTHY / COMPLETED</div>
          <div className="mt-2 text-2xl text-primary">
            {
              rows.filter((r) =>
                ["Healthy", "Active", "Verified", "Completed", "Success"].includes(r.status),
              ).length
            }
          </div>
        </div>
        <div className="border-l border-border pl-8">
          <div className="eyebrow">NEEDS ATTENTION</div>
          <div className="mt-2 text-2xl text-warning">
            {
              rows.filter((r) =>
                ["Degraded", "Missing", "Failed", "Pending", "Warning"].includes(r.status),
              ).length
            }
          </div>
        </div>
      </div>
      {section === "dns" && (
        <div className="mt-5 flex items-start gap-3 border-b border-border pb-5 text-xs">
          <TriangleAlert className="size-4 text-warning" />
          <div>
            <p>Ownership, DNS readiness, provisioning, and delivery are separate checks.</p>
            <p className="mt-2 text-muted-foreground">
              Values are read live from the DevMail backend; nothing is assumed.
            </p>
          </div>
        </div>
      )}
      <div className="toolbar">
        <div className="relative w-full max-w-[280px]">
          <Search className="absolute left-3 top-3 size-3.5 text-muted-foreground" />
          <input
            className="pl-9"
            aria-label={`Search ${section}`}
            placeholder={`Search ${config.title.toLowerCase()}…`}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
        <Select value={filter} onValueChange={setFilter}>
          <SelectTrigger aria-label="Filter by status" className="h-8 w-full max-w-[170px] text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="All statuses">All statuses</SelectItem>
            {[...new Set(rows.map((r) => r.status))].map((s) => (
              <SelectItem key={s} value={s}>
                {s}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="ml-auto flex gap-2">
          {["servers", "domains", "dns"].includes(section) && (
            <Button variant="outline" size="sm" onClick={runChecks} disabled={busy}>
              <RefreshCw className={busy ? "animate-spin" : ""} />
              {busy ? "Checking…" : "Run checks"}
            </Button>
          )}
          <Button
            variant="ghost"
            size="icon"
            title="Export CSV"
            aria-label="Export CSV"
            onClick={exportCsv}
          >
            <Download />
          </Button>
        </div>
      </div>
      <div className="no-scrollbar overflow-x-auto">
        <table className="data-table">
          <thead>
            <tr>
              {config.columns.map((c) => (
                <th key={c[0]}>{c[1]}</th>
              ))}
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {shown.map((r) => (
              <tr key={r.id}>
                {config.columns.map(([key]) => (
                  <td key={key}>
                    {key === "status" ? (
                      <Status value={r.status} />
                    ) : key === "name" ? (
                      <Button
                        variant="ghost"
                        onClick={() => setDetail(r)}
                        className="h-auto justify-start px-0 text-xs font-normal"
                      >
                        {r.name}
                      </Button>
                    ) : (
                      <span
                        className={
                          ["ip", "value", "started", "progress"].includes(key)
                            ? "font-mono text-[10px] text-muted-foreground"
                            : "text-muted-foreground"
                        }
                      >
                        {r[key] || "—"}
                        {key === "hostname" && r["hostname"]}
                      </span>
                    )}
                  </td>
                ))}
                <td>
                  <div className="flex gap-1">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-7"
                      title="Inspect"
                      aria-label={`Inspect ${r.name}`}
                      onClick={() => setDetail(r)}
                    >
                      <Eye className="size-3.5" />
                    </Button>
                    {config.fields.length > 0 && (
                      <>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-7"
                          title="Edit"
                          aria-label={`Edit ${r.name}`}
                          onClick={() => {
                            setError("");
                            setEditor({ ...r });
                          }}
                        >
                          <Pencil className="size-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-7 text-muted-foreground"
                          title="Remove"
                          aria-label={`Remove ${r.name}`}
                          onClick={() => setDeletion(r)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {shown.length === 0 && (
          <div className="py-20 text-center">
            <Search className="mx-auto mb-4 size-6 text-muted-foreground" />
            <p className="text-sm">No {config.title.toLowerCase()} found</p>
            <p className="mt-2 text-xs text-muted-foreground">Try a different search or status.</p>
            <Button
              variant="ghost"
              className="mt-3"
              onClick={() => {
                setQuery("");
                setFilter("All statuses");
              }}
            >
              Clear filters
            </Button>
          </div>
        )}
      </div>
      <div className="mt-4 flex justify-between font-mono text-[10px] text-muted-foreground">
        <span>
          {shown.length} OF {rows.length} RECORDS
        </span>
        <span>{backend === "online" ? "LIVE DATA" : "OFFLINE"}</span>
      </div>
      <Dialog
        open={!!editor}
        onOpenChange={(open) => {
          if (!open) setEditor(null);
        }}
      >
        <DialogContent className="dialog-scroll" aria-describedby={undefined}>
          <DialogTitle>
            {editor && rows.some((r) => r.id === editor.id) ? "Edit" : config.action || "Add"}{" "}
            {editor && rows.some((r) => r.id === editor.id) ? config.singular : ""}
          </DialogTitle>
          {editor && (
            <form onSubmit={submit} className="grid grid-cols-2 gap-4">
              {config.fields.map((f) => (
                <label
                  key={f.key}
                  className={
                    ["name", "destination", "password", "value"].includes(f.key) ? "col-span-2" : ""
                  }
                >
                  {f.label}
                  {f.required ? " *" : ""}
                  {f.options ? (
                    <Select
                      value={editor[f.key] || f.options[0]}
                      onValueChange={(v) => setEditor({ ...editor, [f.key]: v })}
                    >
                      <SelectTrigger aria-label={f.label} className="mt-2 w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {f.options.map((o) => (
                          <SelectItem key={o} value={o}>
                            {o}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  ) : (
                    <input
                      required={
                        f.required &&
                        !(f.key === "password" && rows.some((r) => r.id === editor.id))
                      }
                      type={f.type || "text"}
                      min={f.type === "number" ? 1 : undefined}
                      placeholder={f.placeholder}
                      value={editor[f.key] || ""}
                      autoComplete={f.type === "password" ? "new-password" : "off"}
                      onChange={(e) => setEditor({ ...editor, [f.key]: e.target.value })}
                    />
                  )}
                </label>
              ))}
              {section === "mailboxes" && (
                <Button
                  type="button"
                  variant="outline"
                  className="col-span-2"
                  onClick={() => {
                    const values = new Uint32Array(5);
                    crypto.getRandomValues(values);
                    setEditor({
                      ...editor,
                      password: Array.from(values, (v) => v.toString(36)).join(""),
                    });
                    notify("Password generated. It will be saved with the mailbox.");
                  }}
                >
                  <KeyRound />
                  Generate strong password
                </Button>
              )}
              {section === "servers" && (
                <p className="col-span-2 text-xs leading-5 text-warning">
                  Registering an address does not prove ownership. Enrollment and privileged
                  provisioning require separate explicit authorization. Do not enter private SSH
                  keys here.
                </p>
              )}
              {error && (
                <p className="col-span-2 text-xs text-destructive" role="alert">
                  {error}
                </p>
              )}
              <div className="col-span-2 mt-3 flex justify-end gap-2 border-t border-border pt-4">
                <Button type="button" variant="ghost" onClick={() => setEditor(null)}>
                  Cancel
                </Button>
                <Button type="submit">
                  Save {config.singular}
                  <ArrowRight />
                </Button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!detail}
        onOpenChange={(open) => {
          if (!open) setDetail(null);
        }}
      >
        <DialogContent className="dialog-scroll">
          <DialogTitle className="break-all pr-5">{detail?.name}</DialogTitle>
          <DialogDescription className="capitalize">{config.singular} details</DialogDescription>
          {detail && (
            <>
              <Status value={detail.status} />
              {Object.entries(detail)
                .filter(([k]) => !["id", "name", "status", "password"].includes(k))
                .map(([k, v]) => (
                  <div
                    key={k}
                    className="grid grid-cols-[110px_minmax(0,1fr)_auto] items-start gap-3 border-b border-border py-2 text-xs"
                  >
                    <span className="capitalize text-muted-foreground">{k}</span>
                    <span className="break-all">{v || "—"}</span>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="size-6"
                      aria-label={`Copy ${k}`}
                      onClick={() => copy(v)}
                    >
                      <Copy className="size-3" />
                    </Button>
                  </div>
                ))}
              {section === "domains" && (
                <>
                  <div className="border-b border-border py-3">
                    <p className="text-xs">Ownership TXT challenge</p>
                    {detail["verificationName"] && detail["verificationToken"] ? (
                      <>
                        <p className="mt-2 break-all font-mono text-[10px] text-primary">
                          {detail["verificationName"]}
                        </p>
                        <div className="mt-2 flex items-center justify-between gap-2">
                          <code className="break-all text-[10px]">
                            relay-verify={detail["verificationToken"]}
                          </code>
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="Copy verification challenge"
                            onClick={() =>
                              copy(
                                `${detail["verificationName"]} relay-verify=${detail["verificationToken"]}`,
                              )
                            }
                          >
                            <Copy />
                          </Button>
                        </div>
                      </>
                    ) : (
                      <p className="mt-2 text-xs text-muted-foreground">
                        No verification challenge has been issued yet. Run checks to request one.
                      </p>
                    )}
                  </div>
                  <Button
                    variant="outline"
                    onClick={() => verifyOwnership(detail.id)}
                    disabled={busy}
                  >
                    Verify ownership
                    <RefreshCw />
                  </Button>
                </>
              )}
              {section === "servers" && (
                <>
                  <p className="mt-3 eyebrow">LAST HEALTH CHECK</p>
                  <p className="text-xs leading-6 text-muted-foreground">
                    Last check: {detail["lastCheck"]}
                    <br />
                    Last error: {detail["checkError"]}
                  </p>
                  <Button
                    variant="outline"
                    onClick={async () => {
                      try {
                        await api.toggleServerMaintenance(
                          detail.id,
                          detail["maintenance"] !== "Enabled",
                        );
                        refresh("servers");
                        setDetail(null);
                        notify("Maintenance mode updated.");
                      } catch (e) {
                        notify(describe(e));
                      }
                    }}
                  >
                    {detail["maintenance"] === "Enabled"
                      ? "Disable maintenance mode"
                      : "Enable maintenance mode"}
                  </Button>
                </>
              )}
              {section === "mailboxes" && (
                <>
                  <p className="eyebrow mt-3">MAIL CLIENT CONFIGURATION</p>
                  <p className="text-xs leading-6 text-muted-foreground">
                    IMAP: mail.{detail.name.split("@")[1]} · 993 · TLS
                    <br />
                    SMTP: mail.{detail.name.split("@")[1]} · 587 · STARTTLS
                    <br />
                    Username: full email address
                  </p>
                  <Button
                    variant="outline"
                    onClick={async () => {
                      const values = new Uint32Array(5);
                      crypto.getRandomValues(values);
                      const password = Array.from(values, (v) => v.toString(36)).join("");
                      try {
                        await api.updateMailbox(detail.id, { password });
                        try {
                          await navigator.clipboard.writeText(password);
                        } catch {
                          /* clipboard unavailable */
                        }
                        notify("New mailbox password set and copied to the clipboard.");
                      } catch (e) {
                        notify(describe(e));
                      }
                    }}
                  >
                    Reset password
                    <KeyRound />
                  </Button>
                </>
              )}
              {section === "jobs" && (
                <>
                  {detail["error"] && detail["error"] !== "-" && (
                    <p className="mt-2 break-all text-xs text-warning">{detail["error"]}</p>
                  )}
                  <Button
                    variant="outline"
                    onClick={async () => {
                      try {
                        await api.retryJob(detail.id);
                        refresh("jobs");
                        setDetail(null);
                        notify("Job retry queued.");
                      } catch (e) {
                        notify(describe(e));
                      }
                    }}
                  >
                    Retry job
                    <RefreshCw />
                  </Button>
                </>
              )}
              {section === "backups" && (
                <>
                  <p className="text-xs leading-6 text-warning">
                    Restoring a backup can overwrite mail and configuration. The target, restore
                    point, and authorization must be verified before execution.
                  </p>
                  <Button
                    variant="outline"
                    onClick={() => notify("Restore is not available through the DevMail API yet.")}
                  >
                    Review restore prerequisites
                  </Button>
                </>
              )}
            </>
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
          <DialogTitle>Remove {config.singular}?</DialogTitle>
          <DialogDescription>
            This permanently removes {deletion?.name} from the DevMail backend. This cannot be
            undone.
          </DialogDescription>
          <div className="mt-3 flex justify-end gap-3">
            <Button variant="ghost" onClick={() => setDeletion(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                if (!deletion) return;
                remove(section, deletion.id);
                setDeletion(null);
                notify(`${config.singular} removed.`);
              }}
            >
              Remove {config.singular}
              <Trash2 />
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Ban, Play, RefreshCw, RotateCcw, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { api, type Job } from "@/lib/api";
import { useMail } from "../store";
import { Status } from "../shared/visualizations";

const KINDS = [
  "domain.provision",
  "dns.apply",
  "mailbox.create",
  "alias.provision",
  "server.dns_rebuild",
];

function statusLabel(status: string): string {
  switch (status) {
    case "succeeded":
      return "Completed";
    case "running":
      return "Active";
    case "failed":
      return "Failed";
    case "cancelled":
      return "Suspended";
    default:
      return "Pending";
  }
}

export function JobsPage() {
  const { backend, notify } = useMail();
  const queryClient = useQueryClient();
  const [kind, setKind] = useState(KINDS[0]!);
  const [busy, setBusy] = useState(false);
  const online = backend === "online";

  const jobs = useQuery({
    queryKey: ["jobs"],
    queryFn: api.listJobs,
    retry: false,
    enabled: online,
    refetchInterval: 5_000,
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["jobs"] });

  const enqueue = async () => {
    setBusy(true);
    try {
      await api.enqueueJob({ kind, payload: { requestedBy: "console" } });
      notify(`Queued a ${kind} job.`);
      refresh();
    } catch (err) {
      notify(err instanceof Error ? err.message : "Could not enqueue the job.");
    } finally {
      setBusy(false);
    }
  };

  const retry = async (id: string) => {
    try {
      await api.retryJob(id);
      notify("Job queued for retry.");
      refresh();
    } catch (err) {
      notify(err instanceof Error ? err.message : "Could not retry the job.");
    }
  };

  const cancel = async (id: string) => {
    try {
      await api.cancelJob(id);
      notify("Job cancelled.");
      refresh();
    } catch (err) {
      notify(err instanceof Error ? err.message : "Could not cancel the job.");
    }
  };

  if (!online) {
    return (
      <div className="page-enter">
        <div className="eyebrow mb-3">
          <span className="text-primary">WORKSPACE</span> / JOBS
        </div>
        <h1 className="page-title">Provisioning queue.</h1>
        <div className="mt-6 flex items-start gap-3 border-y border-border py-5 text-xs">
          <TriangleAlert className="size-4 text-warning" />
          <div>
            <p>The job queue requires the DevMail API.</p>
            <p className="mt-2 text-muted-foreground">
              Start the backend and reload to inspect and control provisioning work.
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="page-enter">
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4">
        <div className="min-w-0">
          <div className="eyebrow mb-3">
            <span className="text-primary">WORKSPACE</span> / JOBS
          </div>
          <h1 className="page-title">Provisioning queue.</h1>
          <p className="mt-3 text-[13px] text-muted-foreground">
            Every asynchronous task, its attempts, and its outcome.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <select
            aria-label="Job kind"
            className="h-8 rounded border border-border bg-transparent px-2 text-xs"
            value={kind}
            onChange={(e) => setKind(e.target.value)}
          >
            {KINDS.map((k) => (
              <option key={k} value={k}>
                {k}
              </option>
            ))}
          </select>
          <Button size="sm" disabled={busy} onClick={enqueue}>
            <Play />
            Enqueue
          </Button>
          <Button variant="outline" size="sm" onClick={refresh}>
            <RefreshCw />
            Refresh
          </Button>
        </div>
      </div>

      <div className="no-scrollbar mt-8 overflow-x-auto">
        <table className="data-table">
          <thead>
            <tr>
              <th>Kind</th>
              <th>Status</th>
              <th>Attempts</th>
              <th>Error</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {(jobs.data ?? []).map((j: Job) => (
              <tr key={j.id}>
                <td className="font-mono text-[11px]">{j.kind}</td>
                <td>
                  <Status value={statusLabel(j.status)} />
                </td>
                <td className="font-mono text-[10px] text-muted-foreground">
                  {j.attempts}/{j.maxAttempts}
                </td>
                <td className="max-w-[280px] break-all text-[11px] text-muted-foreground">
                  {j.error || "—"}
                </td>
                <td className="font-mono text-[10px] text-muted-foreground">{j.createdAt}</td>
                <td>
                  <div className="flex gap-1">
                    {(j.status === "failed" || j.status === "cancelled") && (
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-7"
                        title="Retry"
                        aria-label="Retry job"
                        onClick={() => retry(j.id)}
                      >
                        <RotateCcw className="size-3.5" />
                      </Button>
                    )}
                    {(j.status === "queued" || j.status === "retrying") && (
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-7 text-muted-foreground"
                        title="Cancel"
                        aria-label="Cancel job"
                        onClick={() => cancel(j.id)}
                      >
                        <Ban className="size-3.5" />
                      </Button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {(jobs.data ?? []).length === 0 && (
          <div className="py-16 text-center text-sm text-muted-foreground">No jobs yet.</div>
        )}
      </div>
    </div>
  );
}

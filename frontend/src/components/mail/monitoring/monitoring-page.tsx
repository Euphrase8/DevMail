import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
export function MonitoringPage() {
  const summary = useQuery({ queryKey: ["dashboard"], queryFn: api.dashboard });
  return (
    <div className="space-y-6">
      <h1 className="page-title">Monitoring</h1>
      {summary.isError && <p role="alert">Monitoring service unavailable.</p>}
      <div className="grid grid-cols-2 lg:grid-cols-4 border-y border-border py-6">
        {["Delivery rate", "Mail queue", "Spam blocked", "TLS expiry"].map((label) => (
          <div className="metric" key={label}>
            <p>{label}</p>
            <p className="text-xl">Unavailable</p>
            <p className="text-xs text-muted-foreground">
              External mail-engine telemetry is not connected.
            </p>
          </div>
        ))}
      </div>
      {summary.data && (
        <p>
          {summary.data.servers.healthy} healthy � {summary.data.servers.degraded} degraded �{" "}
          {summary.data.servers.offline} offline � {summary.data.servers.pending} pending
        </p>
      )}
      <p>
        Reachability checks measure the configured SMTP endpoint. They do not establish successful
        delivery, IMAP health, disk usage, or certificate expiry.
      </p>
    </div>
  );
}

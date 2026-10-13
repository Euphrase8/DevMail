import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
export function Overview() {
  const summary = useQuery({ queryKey: ["dashboard"], queryFn: api.dashboard });
  return (
    <div className="space-y-8">
      <div className="flex justify-between">
        <h1 className="page-title">Workspace overview</h1>
        <Button onClick={() => void summary.refetch()}>Refresh</Button>
      </div>
      {summary.isPending && <p>Loading workspace…</p>}
      {summary.isError && (
        <p role="alert">Service unavailable. Workspace metrics could not be loaded.</p>
      )}
      {summary.data && (
        <>
          <div className="grid grid-cols-2 lg:grid-cols-4 border-y border-border py-6">
            {[
              ["Servers", summary.data.servers.total],
              ["Domains", summary.data.domains.total],
              ["Mailboxes", summary.data.mailboxes],
              ["Aliases", summary.data.aliases],
            ].map(([label, value]) => (
              <div className="metric" key={label}>
                <p className="eyebrow">{label}</p>
                <p className="metric-number">{value}</p>
              </div>
            ))}
          </div>
          <p>
            {summary.data.servers.healthy} healthy servers · {summary.data.domains.verified}{" "}
            verified domains
          </p>
          <h2 className="section-heading">Recent activity</h2>
          {summary.data.recent.map((item) => (
            <p key={item.id} className="activity-row">
              {item.createdAt} · {item.actor || "System"} · {item.action} · {item.resource}
            </p>
          ))}
        </>
      )}
      <div className="flex gap-4">
        <Button asChild>
          <Link to="/servers">Manage servers</Link>
        </Button>
        <Button asChild variant="outline">
          <Link to="/domains">Manage domains</Link>
        </Button>
      </div>
      <p className="text-sm text-muted-foreground">
        Delivery telemetry is unavailable until an external mail engine supplies measurements.
      </p>
    </div>
  );
}

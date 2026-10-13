import { useQuery } from "@tanstack/react-query";
import { Activity, TriangleAlert } from "lucide-react";
import { api, type Server } from "@/lib/api";
import { useMail } from "../store";
import { Status } from "../shared/visualizations";

function healthStatus(status: string): string {
  switch (status) {
    case "healthy":
      return "Healthy";
    case "degraded":
      return "Degraded";
    case "offline":
      return "Failed";
    default:
      return "Pending";
  }
}

function ServerHealthRow({ server }: { server: Server }) {
  const { data, isError, isPending } = useQuery({
    queryKey: ["server-health", server.id],
    queryFn: () => api.getServerHealth(server.id, 12),
    retry: false,
  });

  return (
    <tr>
      <td className="text-[12px]">{server.name}</td>
      <td className="font-mono text-[10px] text-muted-foreground">{server.hostname}</td>
      <td>
        <Status value={healthStatus(server.status)} />
      </td>
      <td className="font-mono text-[10px] text-muted-foreground">
        {isPending ? "…" : isError ? "unavailable" : `${(data ?? []).length} samples`}
      </td>
      <td>
        <div className="flex items-center gap-1">
          {(data ?? [])
            .slice()
            .reverse()
            .map((p, i) => (
              <span
                key={i}
                title={`${p.status} · ${p.latencyMs}ms · ${p.checkedAt}`}
                className={`inline-block h-3 w-1.5 rounded-sm ${
                  p.status === "healthy"
                    ? "bg-primary"
                    : p.status === "degraded"
                      ? "bg-warning"
                      : "bg-destructive"
                }`}
              />
            ))}
        </div>
      </td>
    </tr>
  );
}

export function ServerHealthPanel() {
  const { backend } = useMail();
  const online = backend === "online";
  const servers = useQuery({
    queryKey: ["servers"],
    queryFn: api.listServers,
    retry: false,
    enabled: online,
  });

  return (
    <section className="py-8">
      <div className="mb-5 flex items-center gap-2.5">
        <Activity className="size-4 text-muted-foreground" />
        <h2 className="section-heading">Server health history</h2>
      </div>
      {!online ? (
        <p className="flex items-center gap-2 text-xs text-warning">
          <TriangleAlert className="size-3.5" />
          Connect the API to record server health over time.
        </p>
      ) : (servers.data ?? []).length === 0 ? (
        <p className="text-xs text-muted-foreground">No servers are registered yet.</p>
      ) : (
        <div className="no-scrollbar overflow-x-auto">
          <table className="data-table">
            <thead>
              <tr>
                <th>Server</th>
                <th>Hostname</th>
                <th>Status</th>
                <th>Samples</th>
                <th>Recent checks</th>
              </tr>
            </thead>
            <tbody>
              {(servers.data ?? []).map((s) => (
                <ServerHealthRow key={s.id} server={s} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

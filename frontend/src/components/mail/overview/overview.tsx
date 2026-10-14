import { useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  ArrowUpRight,
  Clock3,
  Globe,
  Inbox,
  Plus,
  RefreshCw,
  Server,
  ShieldCheck,
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
import { useMail } from "../store";
import { InfrastructureMap, Status, TrafficChart, ViewLink } from "../shared/visualizations";
function timeAgo(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const seconds = Math.max(0, Math.floor((Date.now() - then) / 1000));
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} hr ago`;
  const days = Math.floor(hours / 24);
  return `${days} d ago`;
}
export function Overview() {
  const [period, setPeriod] = useState("24 hours");
  const [refreshing, setRefreshing] = useState(false);
  const { data, notify, dashboard, backend, refresh } = useMail();
  const refreshOverview = () => {
    setRefreshing(true);
    refresh("dashboard");
    notify("Refreshing overview…");
    setTimeout(() => setRefreshing(false), 700);
  };
  const servers = dashboard?.servers;
  const unhealthy = servers ? servers.degraded + servers.offline + servers.pending : 0;
  const attention = dashboard
    ? dashboard.domains.pending + dashboard.domains.degraded + dashboard.domains.suspended
    : 0;
  const metrics = [
    {
      label: "MAIL SERVERS",
      value: servers?.total ?? 0,
      foot: `${servers?.healthy ?? 0} healthy`,
      icon: Server,
      change: `${unhealthy} need attention`,
    },
    {
      label: "DOMAINS",
      value: dashboard?.domains.total ?? 0,
      foot: `${dashboard?.domains.verified ?? 0} verified`,
      icon: Globe,
      change: `${dashboard?.domains.pending ?? 0} pending`,
    },
    {
      label: "MAILBOXES",
      value: dashboard?.mailboxes ?? 0,
      foot: `${dashboard?.users ?? 0} users`,
      icon: Inbox,
      change: `${dashboard?.aliases ?? 0} aliases`,
    },
    {
      label: "ALIASES",
      value: dashboard?.aliases ?? 0,
      foot: `${dashboard?.domains.active ?? 0} active domains`,
      icon: ArrowUpRight,
      change: `${dashboard?.users ?? 0} seats`,
    },
  ];
  return (
    <div className="page-enter relative isolate">
      <img
        src="/brand/devmail-art-coral.svg"
        alt=""
        aria-hidden
        className="overview-art pointer-events-none absolute right-0 top-0 -z-10 hidden h-[420px] w-auto opacity-[0.13] dark:opacity-[0.22] lg:block"
      />
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-4">
        <div className="min-w-0">
          <div className="eyebrow mb-3 flex items-center gap-2">
            <span className="text-primary">WORKSPACE</span>
            <span>/</span>OVERVIEW
          </div>
          <h1 className="page-title">
            Your mail, <span className="text-primary">in sync.</span>
          </h1>
          <p className="mt-3 text-[13px] text-muted-foreground">
            A clear view of everything that keeps your conversations moving.
          </p>
        </div>
        <div className="mt-6 hidden items-center gap-2 sm:flex">
          <Button variant="outline" size="sm" onClick={refreshOverview} title="Refresh overview">
            <RefreshCw className={`size-3 ${refreshing ? "animate-spin" : ""}`} />
            Refresh
          </Button>
          <Button size="sm" asChild>
            <Link to="/servers" search={{ action: "new" }}>
              {" "}
              <Plus className="size-3" />
              Add server
            </Link>
          </Button>
        </div>
      </div>
      <div className="mt-7 flex items-center justify-between gap-3 border-y border-border py-3">
        <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-[11px]">
          <span className="flex items-center gap-2">
            <span className="live-dot" />
            {dashboard
              ? unhealthy > 0
                ? "Attention needed"
                : "Infrastructure running smoothly"
              : "Connecting to backend…"}
          </span>
          {servers && (
            <span className="hidden text-muted-foreground sm:inline">
              {servers.healthy} of {servers.total} servers healthy
            </span>
          )}
        </div>
        <span className="flex shrink-0 items-center gap-1.5 text-[10px] text-muted-foreground">
          <Clock3 className="size-3" />
          {backend === "online" ? "Live snapshot" : "Offline"}
        </span>
      </div>
      <div className="grid grid-cols-2 gap-y-3 py-7 lg:grid-cols-4">
        {metrics.map((m) => (
          <div className="metric" key={m.label}>
            <div className="flex items-center justify-between">
              <span className="eyebrow">{m.label}</span>
              <m.icon className="size-3.5 text-muted-foreground" />
            </div>
            <div className="metric-number mt-2">{m.value}</div>
            <div className="mb-3 text-[10px] text-primary">{m.change}</div>
            <div className="text-[11px] text-muted-foreground">{m.foot}</div>
          </div>
        ))}
      </div>
      <div className="grid border-y border-border lg:grid-cols-[1.2fr_1fr]">
        <section className="min-w-0 py-6 lg:border-r lg:border-border lg:pr-7">
          <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center">
            <div>
              <h2 className="section-heading">Mail traffic</h2>
              <p className="mt-1 text-[11px] text-muted-foreground">
                Every message. A little more connected.
              </p>
            </div>
            <Select value={period} onValueChange={setPeriod}>
              <SelectTrigger aria-label="Traffic time range" className="h-7 w-24 px-2 text-[10px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="24 hours">24 hours</SelectItem>
                <SelectItem value="7 days">7 days</SelectItem>
                <SelectItem value="30 days">30 days</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <TrafficChart period={period} />
          <div className="mt-2 flex items-center justify-between">
            <div className="flex gap-5 text-[10px] text-muted-foreground">
              <span className="flex items-center gap-2">
                <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                Delivered
              </span>
              <span className="flex items-center gap-2">
                <span className="h-1.5 w-1.5 rounded-full bg-info" />
                Received
              </span>
            </div>
            <span className="hidden font-mono text-[9px] text-muted-foreground sm:inline">UTC</span>
          </div>
        </section>
        <section className="min-w-0 border-t border-border py-6 lg:border-t-0 lg:pl-7">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="section-heading">Connected infrastructure</h2>
              <p className="mt-1 text-[11px] text-muted-foreground">
                Many parts. One seamless system.
              </p>
            </div>
            <ViewLink to="/monitoring" label="Explore" />
          </div>
          <InfrastructureMap />
          <div className="flex justify-between border-t border-border pt-3 font-mono text-[9px] text-muted-foreground">
            <span>
              {dashboard?.servers.total ?? 0} SERVERS <span className="mx-1 text-border">/</span>{" "}
              {dashboard?.domains.total ?? 0} DOMAINS
            </span>
            <span className="flex items-center gap-1.5 text-primary">
              <span className="live-dot" />
              ENCRYPTED END-TO-END
            </span>
          </div>
        </section>
      </div>
      <section className="py-6">
        <div className="mb-5 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <h2 className="section-heading">Your servers</h2>
            <span className="font-mono text-[10px] text-muted-foreground">
              {String((data["servers"] ?? []).length).padStart(2, "0")}
            </span>
          </div>
          <ViewLink to="/servers" label="Manage servers" />
        </div>
        <div className="no-scrollbar overflow-x-auto">
          <table className="data-table">
            <thead>
              <tr>
                {["SERVER / HOSTNAME", "REGION", "DOMAINS", "RESOURCE USAGE", "STATUS", ""].map(
                  (h, i) => (
                    <th key={i}>{h}</th>
                  ),
                )}
              </tr>
            </thead>
            <tbody>
              {(data["servers"] ?? []).map((s) => (
                <tr key={s.id}>
                  <td>
                    <Link to="/servers" className="flex items-center gap-3 hover:text-primary">
                      <span className="grid h-8 w-8 place-items-center rounded border border-border">
                        <Server className="size-3.5 text-muted-foreground" />
                      </span>
                      <span>
                        <span className="block text-[12px]">{s.name}</span>
                        <span className="mt-1 block font-mono text-[10px] text-muted-foreground">
                          {s["hostname"] ?? ""}
                        </span>
                      </span>
                    </Link>
                  </td>
                  <td className="text-muted-foreground">{s["location"] ?? "—"}</td>
                  <td>
                    {s["domains"] ?? "—"}
                    <span className="ml-1 text-muted-foreground">domains</span>
                  </td>
                  <td className="text-muted-foreground">{s["usage"] ?? "—"}</td>
                  <td>
                    <Status value={s.status} />
                  </td>
                  <td>
                    <Button
                      asChild
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6"
                      aria-label={`Inspect ${s.name}`}
                    >
                      <Link to="/servers">
                        <ArrowUpRight className="size-3" />
                      </Link>
                    </Button>
                  </td>
                </tr>
              ))}
              {(data["servers"] ?? []).length === 0 && (
                <tr>
                  <td colSpan={6} className="py-10 text-center text-xs text-muted-foreground">
                    No servers registered yet. Add one to get started.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
      <div className="grid gap-8 border-t border-border pt-6 lg:grid-cols-[1.2fr_1fr]">
        <section className="min-w-0 lg:border-r lg:border-border lg:pr-7">
          <div className="mb-2 flex items-center justify-between">
            <h2 className="section-heading">Recent activity</h2>
            <ViewLink to="/audit" />
          </div>
          {(dashboard?.recent ?? []).map((a) => (
            <div
              key={a.id}
              className="activity-row grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3"
            >
              <ShieldCheck className="size-3.5 text-muted-foreground" />
              <div>
                <div className="text-[11px]">{a.action}</div>
                <div className="mt-1 text-[10px] text-muted-foreground">
                  {a.actor} · {a.resource}
                </div>
              </div>
              <span className="font-mono text-[9px] text-muted-foreground">
                {timeAgo(a.createdAt)}
              </span>
            </div>
          ))}
          {(dashboard?.recent ?? []).length === 0 && (
            <p className="py-4 text-[11px] text-muted-foreground">
              No recent activity recorded yet.
            </p>
          )}
        </section>
        <section>
          <div className="mb-5 flex items-center justify-between">
            <h2 className="section-heading">Needs your attention</h2>
            <span className="font-mono text-[10px] text-warning">
              {String(attention).padStart(2, "0")} OPEN
            </span>
          </div>
          {attention > 0 ? (
            <div className="flex items-start gap-3">
              <TriangleAlert className="mt-1 size-4 text-warning" />
              <div>
                <h3 className="text-[12px]">A few domains need review.</h3>
                <p className="mt-2 max-w-80 text-[11px] leading-5 text-muted-foreground">
                  {attention} domain{attention === 1 ? "" : "s"} are pending verification or
                  degraded. Review DNS and ownership to keep mail flowing.
                </p>
                <Button
                  variant="ghost"
                  asChild
                  size="sm"
                  className="mt-3 h-7 px-0 text-[11px] text-primary"
                >
                  <Link to="/dns">
                    Review DNS configuration
                    <ArrowRight className="size-3" />
                  </Link>
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex items-start gap-3">
              <ShieldCheck className="mt-1 size-4 text-primary" />
              <div>
                <h3 className="text-[12px]">Everything looks healthy.</h3>
                <p className="mt-2 max-w-80 text-[11px] leading-5 text-muted-foreground">
                  No domains currently need attention.
                </p>
              </div>
            </div>
          )}
        </section>
      </div>
    </div>
  );
}

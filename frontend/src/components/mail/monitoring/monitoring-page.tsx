import { useState } from "react";
import { RefreshCw, ShieldCheck, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useMail } from "../store";
import { InfrastructureMap, Status, TrafficChart } from "../shared/visualizations";
export function MonitoringPage() {
  const { data, notify } = useMail();
  const [period, setPeriod] = useState("24 hours");
  const [tab, setTab] = useState("Delivery");
  return (
    <div className="page-enter">
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
        <div>
          <div className="eyebrow mb-3">
            <span className="text-primary">OPERATIONS</span> / MONITORING
          </div>
          <h1 className="page-title">Nothing out of sight.</h1>
          <p className="mt-3 text-[13px] text-muted-foreground">
            Infrastructure health, delivery, and security. In one continuous view.
          </p>
        </div>
        <Button
          variant="outline"
          size="icon"
          aria-label="Refresh monitoring"
          onClick={() => notify("Sample monitoring refreshed. Live probes are not connected.")}
        >
          <RefreshCw />
        </Button>
      </div>
      <div className="mt-8 grid grid-cols-2 border-y border-border py-6 lg:grid-cols-4">
        {[
          ["DELIVERY RATE", "99.8%", "+0.2% this week"],
          ["MAIL QUEUE", "24", "Within normal range"],
          ["SPAM BLOCKED", "1,284", "9.1% of incoming"],
          ["TLS EXPIRY", "62 days", "Next: eu-west-01"],
        ].map(([l, v, d]) => (
          <div key={l} className="metric">
            <div className="eyebrow">{l}</div>
            <p className="metric-number mt-2">{v}</p>
            <p className="text-[11px] text-primary">{d}</p>
          </div>
        ))}
      </div>
      <div className="mt-6 flex gap-1 border-b border-border">
        {["Delivery", "Resources", "Services", "Security"].map((t) => (
          <Button
            key={t}
            variant="ghost"
            onClick={() => setTab(t)}
            className={`rounded-none border-b-2 text-xs ${tab === t ? "border-primary text-primary" : "border-transparent text-muted-foreground"}`}
          >
            {t}
          </Button>
        ))}
      </div>
      {tab === "Delivery" ? (
        <div className="grid gap-8 py-7 lg:grid-cols-[1.2fr_1fr]">
          <div>
            <div className="flex justify-between">
              <h2 className="section-heading">Message delivery</h2>
              <Select value={period} onValueChange={setPeriod}>
                <SelectTrigger
                  aria-label="Monitoring time range"
                  className="h-7 w-28 px-2 text-[11px]"
                >
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
          </div>
          <div>
            <h2 className="section-heading mb-5">Infrastructure topology</h2>
            <InfrastructureMap />
          </div>
        </div>
      ) : (
        <div className="overflow-x-auto py-7">
          <table className="data-table">
            <thead>
              <tr>
                <th>Server</th>
                <th>{tab === "Resources" ? "CPU" : "SMTP"}</th>
                <th>{tab === "Resources" ? "Memory" : "IMAP"}</th>
                <th>{tab === "Resources" ? "Disk" : "TLS remaining"}</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {(data["servers"] ?? []).map((s, i) => (
                <tr key={s.id}>
                  <td>{s.name}</td>
                  <td>{tab === "Resources" ? ["24%", "18%", "12%"][i] : "Available"}</td>
                  <td>
                    {tab === "Resources"
                      ? ["3.2 / 8 GB", "2.8 / 8 GB", "1.6 / 4 GB"][i]
                      : "Available"}
                  </td>
                  <td>
                    {tab === "Resources" ? ["38%", "29%", "64%"][i] : `${[84, 78, 62][i]} days`}
                  </td>
                  <td>
                    <Status value={s.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <section className="border-t border-border pt-6">
        <h2 className="section-heading">Open alerts</h2>
        <div className="activity-row grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-4">
          <TriangleAlert className="size-4 text-warning" />
          <div>
            <p className="text-xs">Missing DMARC record</p>
            <p className="mt-2 text-xs text-muted-foreground">
              orbit.design · Domain authentication · Sample warning
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => notify("Sample alert acknowledged. Live alert state is not connected.")}
          >
            Acknowledge
          </Button>
        </div>
        <div className="mt-5 flex items-center gap-2 text-xs text-muted-foreground">
          <ShieldCheck className="size-4 text-primary" />
          Metrics are illustrative; production health requires actual probes and delivery checks.
        </div>
      </section>
    </div>
  );
}

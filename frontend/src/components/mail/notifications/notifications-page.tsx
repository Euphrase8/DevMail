import { useState } from "react";
import { Bell, Check, Mail, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useMail } from "../store";

const events = [
  { id: "delivery", label: "Delivery failures", severity: "critical" },
  { id: "tls", label: "TLS certificate warnings", severity: "warning" },
  { id: "dns", label: "DNS validation warnings", severity: "warning" },
  { id: "server", label: "Server offline or degraded", severity: "critical" },
  { id: "backup", label: "Backup failures", severity: "critical" },
  { id: "quota", label: "Mailbox quota exceeded", severity: "warning" },
] as const;

function Toggle({ on, label, onClick }: { on: boolean; label: string; onClick: () => void }) {
  return (
    <Button
      type="button"
      variant={on ? "default" : "outline"}
      size="sm"
      className="h-7 min-w-16 text-[11px]"
      aria-pressed={on}
      onClick={onClick}
    >
      {label}
      {on && <Check className="size-3" />}
    </Button>
  );
}

export function NotificationsPage() {
  const { settings, setSetting, notify } = useMail();
  const [email, setEmail] = useState(settings["notifications"] === "On");
  const [webhook, setWebhook] = useState(false);
  const [alertEmail, setAlertEmail] = useState("");
  const [webhookUrl, setWebhookUrl] = useState("");
  const [rules, setRules] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(events.map((e) => [`${e.id}-email`, e.severity === "critical"])),
  );

  const toggleRule = (key: string) => setRules((old) => ({ ...old, [key]: !old[key] }));

  const channelsOn = [email, webhook].filter(Boolean).length;
  const rulesOn = Object.values(rules).filter(Boolean).length;

  const save = () => {
    setSetting("notifications", email ? "On" : "Off");
    setSetting("alertEmail", alertEmail);
    setSetting("webhook", webhookUrl);
    notify("Notification preferences saved.");
  };

  return (
    <div className="page-enter">
      <div className="eyebrow mb-3">
        <span className="text-primary">OPERATIONS</span> / NOTIFICATIONS
      </div>
      <h1 className="page-title">Alerts that reach you.</h1>
      <p className="mt-3 text-[13px] text-muted-foreground">
        Choose what matters, where it should land, and how seriously to take it.
      </p>

      <div className="mt-8 grid grid-cols-2 border-y border-border py-6 lg:grid-cols-4">
        {[
          ["TRACKED EVENTS", String(events.length), "Delivery, TLS, DNS, servers, backups"],
          ["CHANNELS ON", String(channelsOn), email ? "Email active" : "None yet"],
          ["RULES ACTIVE", String(rulesOn), `${events.length * 2 - rulesOn} muted`],
          ["SEVERITY", "2", "Warning and critical"],
        ].map(([l, v, d]) => (
          <div key={l} className="metric">
            <div className="eyebrow">{l}</div>
            <p className="metric-number mt-2">{v}</p>
            <p className="text-[11px] text-primary">{d}</p>
          </div>
        ))}
      </div>

      <div className="mt-10 grid gap-10 border-y border-border py-8 lg:grid-cols-[1fr_1.4fr]">
        <section>
          <h2 className="section-heading">Delivery channels</h2>
          <div className="mt-4">
            <div className="settings-row">
              <div>
                <h3 className="text-sm">Email alerts</h3>
                <p>Operational alerts for your mailbox and beyond.</p>
              </div>
              <div className="space-y-2">
                <Toggle on={email} label="Email" onClick={() => setEmail(!email)} />
                {email && (
                  <input
                    type="email"
                    aria-label="Alert recipient email"
                    placeholder="admin@example.com"
                    value={alertEmail}
                    onChange={(e) => setAlertEmail(e.target.value)}
                  />
                )}
              </div>
            </div>
            <div className="settings-row">
              <div>
                <h3 className="text-sm">Webhook</h3>
                <p>Forward events to an internal endpoint, chat, or pager.</p>
              </div>
              <div className="space-y-2">
                <Toggle on={webhook} label="Webhook" onClick={() => setWebhook(!webhook)} />
                {webhook && (
                  <input
                    type="url"
                    aria-label="Webhook URL"
                    placeholder="https://hooks.example.com/devmail"
                    value={webhookUrl}
                    onChange={(e) => setWebhookUrl(e.target.value)}
                  />
                )}
              </div>
            </div>
            <div className="settings-row">
              <div>
                <h3 className="text-sm">In-app banner</h3>
                <p>Shown in the header of this workspace. Always on.</p>
              </div>
              <div className="flex items-center gap-2 text-xs text-primary">
                <Bell className="size-4" />
                Always on
              </div>
            </div>
          </div>
        </section>

        <section>
          <h2 className="section-heading">Event rules</h2>
          <div className="mt-5 overflow-x-auto">
            <div className="min-w-[420px]">
              <div className="grid grid-cols-[minmax(0,1fr)_70px_auto_auto] items-center gap-3 border-b border-border pb-3">
                <span className="eyebrow">EVENT</span>
                <span className="eyebrow">SEVERITY</span>
                <span className="eyebrow text-center">EMAIL</span>
                <span className="eyebrow text-center">WEBHOOK</span>
              </div>
              {events.map((e) => (
                <div
                  key={e.id}
                  className="grid grid-cols-[minmax(0,1fr)_70px_auto_auto] items-center gap-3 border-b border-border py-3 text-xs"
                >
                  <span>{e.label}</span>
                  <span className={e.severity === "critical" ? "text-destructive" : "text-warning"}>
                    {e.severity === "critical" ? "Critical" : "Warning"}
                  </span>
                  <input
                    type="checkbox"
                    aria-label={`Email alert for ${e.label}`}
                    checked={rules[`${e.id}-email`] ?? false}
                    onChange={() => toggleRule(`${e.id}-email`)}
                  />
                  <input
                    type="checkbox"
                    aria-label={`Webhook alert for ${e.label}`}
                    checked={rules[`${e.id}-webhook`] ?? false}
                    onChange={() => toggleRule(`${e.id}-webhook`)}
                  />
                </div>
              ))}
            </div>
          </div>
          <p className="mt-4 flex items-start gap-2 text-[11px] leading-5 text-warning">
            <Mail className="mt-0.5 size-3.5 shrink-0" />
            Notification delivery isn't connected yet. Your preferences are saved and will take
            effect once a delivery service is configured.
          </p>
        </section>
      </div>

      <div className="mt-8 flex items-center justify-between gap-4">
        <p className="text-[11px] text-muted-foreground">
          Preferences are stored by the DevMail backend.
        </p>
        <Button size="sm" onClick={save}>
          <Save />
          Save preferences
        </Button>
      </div>
    </div>
  );
}

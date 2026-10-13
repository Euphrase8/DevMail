import { useState } from "react";
import { KeyRound, Save, ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useMail } from "../store";
const tabs = [
  "General",
  "Mail engine",
  "DNS providers",
  "Security",
  "Notifications",
  "Storage",
  "API & integrations",
];
export function SettingsPage() {
  const { settings, setSetting, notify } = useMail();
  const [tab, setTab] = useState("General");
  const [values, setValues] = useState({ ...settings });
  const input = (key: string, placeholder = "") => (
    <input
      aria-label={key}
      type={key.includes("password") ? "password" : "text"}
      value={values[key] || ""}
      placeholder={placeholder}
      onChange={(e) => setValues({ ...values, [key]: e.target.value })}
    />
  );
  const select = (key: string, options: string[]) => (
    <Select
      value={values[key] || options[0]}
      onValueChange={(v) => setValues({ ...values, [key]: v })}
    >
      <SelectTrigger aria-label={key} className="h-9 w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((v) => (
          <SelectItem key={v} value={v}>
            {v}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
  const row = (title: string, description: string, control: React.ReactNode) => (
    <div className="settings-row" key={title}>
      <div>
        <h3 className="text-sm">{title}</h3>
        <p>{description}</p>
      </div>
      <div>{control}</div>
    </div>
  );
  return (
    <div className="page-enter">
      <div className="eyebrow mb-3">
        <span className="text-primary">WORKSPACE</span> / SETTINGS
      </div>
      <h1 className="page-title">Your workspace, your way.</h1>
      <p className="mt-3 text-[13px] text-muted-foreground">
        Fine-tune the foundations of your mail infrastructure.
      </p>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          Object.entries(values)
            .filter(([k]) => !k.includes("password") && !k.includes("token"))
            .forEach(([k, v]) => setSetting(k, v));
          notify("Sample settings saved for this session. Services are not configured.");
        }}
      >
        <div className="mt-8 flex flex-col lg:h-[calc(100vh-220px)] lg:overflow-hidden">
          <div className="grid lg:h-full lg:grid-cols-[220px_minmax(0,1fr)] lg:gap-9">
            <nav
              aria-label="Settings sections"
              className="shrink-0 lg:border-r lg:border-border lg:pr-9 lg:pt-6"
            >
              {tabs.map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setTab(t)}
                  className={`flex w-full items-center gap-2.5 border-b border-border py-3 pr-2 text-left text-xs transition-colors ${
                    tab === t ? "text-primary" : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  <span
                    className={`h-1.5 w-1.5 rounded-full ${tab === t ? "bg-primary" : "bg-transparent"}`}
                  />
                  {t}
                </button>
              ))}
            </nav>
            <div className="no-scrollbar min-h-0 lg:overflow-y-auto lg:pt-6 lg:pr-2">
              <h2 className="section-heading">{tab}</h2>
              {tab === "General" && (
                <>
                  {row(
                    "Workspace name",
                    "The name displayed across your administration panel.",
                    input("name"),
                  )}
                  {row(
                    "Timezone",
                    "Used for activity, jobs, and scheduled operations.",
                    select("timezone", [
                      "UTC",
                      "Europe/Berlin",
                      "America/New_York",
                      "Africa/Dar_es_Salaam",
                    ]),
                  )}
                  {row(
                    "Log retention",
                    "Number of days to retain administrative audit records.",
                    input("retention", "90"),
                  )}
                </>
              )}
              {tab === "Mail engine" && (
                <>
                  {row(
                    "Mail engine",
                    "Supported administrative adapter.",
                    select("engine", ["Mailcow", "Other — adapter required"]),
                  )}
                  {row(
                    "Administrative URL",
                    "The secure mail-engine API endpoint.",
                    input("engineUrl", "https://mail.example.com/api/v1"),
                  )}
                  {row(
                    "API credential",
                    "Use a restricted key. Never expose credentials in browser code.",
                    <p className="py-2 text-xs text-warning">
                      Credential entry is disabled until a secure service is connected.
                    </p>,
                  )}
                  {row(
                    "Connection status",
                    "No live engine is connected to this preview.",
                    <span className="status status-pending">Not connected</span>,
                  )}
                </>
              )}
              {tab === "DNS providers" && (
                <>
                  {row(
                    "Default provider",
                    "Manual DNS works with any provider.",
                    select("provider", ["Manual", "Cloudflare"]),
                  )}
                  {row(
                    "Cloudflare zone ID",
                    "Select the zone under your control.",
                    input("zone", "Zone identifier"),
                  )}
                  {row(
                    "API token",
                    "Zone-scoped DNS read and edit permissions only.",
                    <p className="py-2 text-xs text-warning">
                      Secure credential storage requires service integration.
                    </p>,
                  )}
                  {row(
                    "Ownership verification",
                    "Require domain control before activation.",
                    <div className="flex items-center gap-2 text-xs text-primary">
                      <ShieldCheck className="size-4" />
                      Always required
                    </div>,
                  )}
                </>
              )}
              {tab === "Security" && (
                <>
                  {row(
                    "Multi-factor authentication",
                    "TOTP enrollment needs a connected authentication service.",
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() =>
                        notify(
                          "MFA enrollment requires authentication integration. It is not enabled.",
                        )
                      }
                    >
                      Set up authenticator
                      <KeyRound />
                    </Button>,
                  )}
                  {row(
                    "Session duration",
                    "Maximum administrator session lifetime.",
                    select("session", ["8 hours", "1 hour", "24 hours"]),
                  )}
                  {row(
                    "Active sessions",
                    "This is a frontend preview, not an authenticated session.",
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => notify("No authenticated sessions are connected to revoke.")}
                    >
                      Revoke all sessions
                    </Button>,
                  )}
                  {row(
                    "Sensitive operations",
                    "Always require confirmation for destructive changes.",
                    <span className="status status-healthy">Confirmation required</span>,
                  )}
                </>
              )}
              {tab === "Notifications" && (
                <>
                  {row(
                    "Email notifications",
                    "Delivery, certificate, and server health alerts.",
                    select("notifications", ["On", "Off"]),
                  )}
                  {row(
                    "Alert email",
                    "Recipient for operational alerts.",
                    <input
                      type="email"
                      aria-label="Alert email"
                      value={values["alertEmail"] || ""}
                      onChange={(e) => setValues({ ...values, alertEmail: e.target.value })}
                      placeholder="admin@example.com"
                    />,
                  )}
                  {row(
                    "Webhook URL",
                    "Optional notification endpoint.",
                    input("webhook", "https://example.com/webhook"),
                  )}
                  {row(
                    "Alert severity",
                    "Minimum severity for notifications.",
                    select("severity", ["Warning and critical", "Critical only", "All events"]),
                  )}
                </>
              )}
              {tab === "Storage" && (
                <>
                  {row(
                    "Backup destination",
                    "Choose a compatible backup target.",
                    select("storage", ["S3-compatible", "Local volume"]),
                  )}
                  {row("Bucket name", "Backup storage bucket.", input("bucket", "relay-backups"))}
                  {row(
                    "Endpoint URL",
                    "Your object storage endpoint.",
                    input("storageUrl", "https://storage.example.com"),
                  )}
                  {row(
                    "Encryption",
                    "Encrypt backup archives before upload.",
                    select("encryption", ["Required", "Required — customer managed key"]),
                  )}
                  {row(
                    "Retention",
                    "Days to keep completed backups.",
                    input("backupRetention", "30"),
                  )}
                </>
              )}
              {tab === "API & integrations" && (
                <>
                  {row(
                    "Control-plane API",
                    "Connection boundary for the continuing implementation.",
                    input("api"),
                  )}
                  {row(
                    "Service status",
                    "Backend services are intentionally outside this frontend scope.",
                    <span className="status status-pending">Not connected</span>,
                  )}
                  {row(
                    "Administrative tokens",
                    "Tokens must be issued and stored by the backend.",
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() =>
                        notify("Token issuance is not connected. No API token was created.")
                      }
                    >
                      Create scoped token
                      <KeyRound />
                    </Button>,
                  )}
                  {row(
                    "Webmail URL",
                    "HTTPS Roundcube or mail-suite webmail endpoint.",
                    input("webmail", "https://mail.example.com/SOGo/"),
                  )}
                </>
              )}
              <div className="mt-8 flex items-center justify-between gap-4 border-t border-border pt-5">
                <p className="text-[11px] text-muted-foreground">
                  Frontend settings only · sensitive credentials are never retained.
                </p>
                <Button type="submit" size="sm">
                  <Save />
                  Save changes
                </Button>
              </div>
            </div>
          </div>
        </div>
      </form>
    </div>
  );
}

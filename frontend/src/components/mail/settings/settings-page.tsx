import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { SecurityPanel } from "../auth/security-panel";
export function SettingsPage() {
  const client = useQueryClient();
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.getSettings });
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const workspace = settings.data?.["workspace"] as { name?: string } | undefined;
  return (
    <div className="space-y-8">
      <h1 className="page-title">Settings</h1>
      {settings.isError && <p role="alert">Settings unavailable.</p>}
      {settings.data && (
        <form
          className="grid gap-4 max-w-xl"
          onSubmit={async (e) => {
            e.preventDefault();
            const data = new FormData(e.currentTarget);
            setBusy(true);
            try {
              await api.updateSettings({
                name: String(data.get("name")),
                timezone: String(data.get("timezone")),
                webmail: String(data.get("webmail")),
                defaultQuotaBytes: Number(data.get("quota")),
                mailEngine: String(data.get("mailEngine")),
              });
              await client.invalidateQueries({ queryKey: ["settings"] });
              setMessage(
                "Settings saved. Engine changes take effect after API restart; an explicit MAIL_ENGINE environment setting takes precedence.",
              );
            } catch (error) {
              setMessage(error instanceof Error ? error.message : "Save failed.");
            } finally {
              setBusy(false);
            }
          }}
        >
          <label>
            Workspace name
            <input
              name="name"
              required
              maxLength={100}
              defaultValue={String(settings.data["name"] ?? workspace?.name ?? "DevMail")}
            />
          </label>
          <label>
            Timezone
            <input
              name="timezone"
              required
              defaultValue={String(settings.data["timezone"] ?? "UTC")}
            />
          </label>
          <label>
            Webmail URL
            <input
              name="webmail"
              type="url"
              defaultValue={String(settings.data["webmail"] ?? "")}
            />
          </label>
          <label>
            Default mailbox quota (bytes)
            <input
              name="quota"
              type="number"
              min={0}
              defaultValue={Number(settings.data["defaultQuotaBytes"] ?? 10737418240)}
            />
          </label>
          <label>
            Mail engine
            <select name="mailEngine" defaultValue={String(settings.data["mailEngine"] ?? "none")}>
              <option value="none">Unmanaged</option>
              <option value="sql">SQL virtual users</option>
            </select>
          </label>
          <Button disabled={busy}>Save workspace settings</Button>
        </form>
      )}
      {message && <p role="status">{message}</p>}
      <SecurityPanel />
    </div>
  );
}

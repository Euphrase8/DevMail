import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Copy, KeyRound, RefreshCw, Send, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api, type DnsRecord } from "@/lib/api";
import { useMail } from "../store";
import { Status } from "../shared/visualizations";

function statusLabel(status: string): string {
  switch (status) {
    case "published":
      return "Verified";
    case "manual":
      return "Pending";
    case "error":
      return "Failed";
    default:
      return "Pending";
  }
}

export function DnsPage() {
  const { backend, notify } = useMail();
  const queryClient = useQueryClient();
  const [domainId, setDomainId] = useState("");
  const [providerId, setProviderId] = useState("");
  const [busy, setBusy] = useState(false);

  const online = backend === "online";

  const domains = useQuery({
    queryKey: ["domains"],
    queryFn: api.listDomains,
    retry: false,
    enabled: online,
  });
  const providers = useQuery({
    queryKey: ["providers"],
    queryFn: api.listProviders,
    retry: false,
    enabled: online,
  });

  const activeDomain = domainId || domains.data?.[0]?.id || "";
  const activeProvider = providerId || providers.data?.[0]?.id || "";

  const records = useQuery({
    queryKey: ["dns-records", activeDomain],
    queryFn: () => api.listDnsRecords(activeDomain),
    retry: false,
    enabled: online && !!activeDomain,
  });

  const published = useMemo(
    () =>
      (records.data ?? []).filter((r) => r.status === "published" || r.status === "manual").length,
    [records.data],
  );

  const copy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      notify("Copied to clipboard.");
    } catch {
      notify("Clipboard access unavailable. Select and copy the value manually.");
    }
  };

  const generateDkim = async () => {
    if (!activeDomain) return;
    setBusy(true);
    try {
      const res = await api.generateDkim(activeDomain, "default");
      notify(`DKIM key generated for selector "${res.selector}".`);
      queryClient.invalidateQueries({ queryKey: ["dns-records", activeDomain] });
    } catch (err) {
      notify(err instanceof Error ? err.message : "Could not generate a DKIM key.");
    } finally {
      setBusy(false);
    }
  };

  const applyRecords = async () => {
    if (!activeDomain || !activeProvider) return;
    setBusy(true);
    try {
      const res = await api.applyDnsRecords(activeDomain, activeProvider);
      const ok = res.results.filter((r) => r.status === "published").length;
      const manual = res.results.filter((r) => r.status === "manual").length;
      const failed = res.results.filter((r) => r.status === "error").length;
      notify(`Apply finished — ${ok} published, ${manual} manual, ${failed} failed.`);
      queryClient.invalidateQueries({ queryKey: ["dns-records", activeDomain] });
    } catch (err) {
      notify(err instanceof Error ? err.message : "Could not apply DNS records.");
    } finally {
      setBusy(false);
    }
  };

  if (!online) {
    return (
      <div className="page-enter">
        <div className="eyebrow mb-3">
          <span className="text-primary">INFRASTRUCTURE</span> / DNS
        </div>
        <h1 className="page-title">DNS records.</h1>
        <div className="mt-6 flex items-start gap-3 border-y border-border py-5 text-xs">
          <TriangleAlert className="size-4 text-warning" />
          <div>
            <p>DNS planning requires the DevMail API.</p>
            <p className="mt-2 text-muted-foreground">
              Start the backend and reload to generate records from your real configuration.
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
            <span className="text-primary">INFRASTRUCTURE</span> / DNS
          </div>
          <h1 className="page-title">Where your DNS meets your mail.</h1>
          <p className="mt-3 text-[13px] text-muted-foreground">
            Records are generated from your real server, domain and DKIM configuration — never
            hard-coded.
          </p>
        </div>
      </div>

      <div className="toolbar mt-6">
        <Select value={activeDomain} onValueChange={setDomainId}>
          <SelectTrigger aria-label="Select domain" className="h-8 w-full max-w-[240px] text-xs">
            <SelectValue placeholder="Select a domain" />
          </SelectTrigger>
          <SelectContent>
            {(domains.data ?? []).map((d) => (
              <SelectItem key={d.id} value={d.id}>
                {d.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={activeProvider} onValueChange={setProviderId}>
          <SelectTrigger aria-label="Select provider" className="h-8 w-full max-w-[220px] text-xs">
            <SelectValue placeholder="Select a provider" />
          </SelectTrigger>
          <SelectContent>
            {(providers.data ?? []).map((p) => (
              <SelectItem key={p.id} value={p.id}>
                {p.name} ({p.kind})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="ml-auto flex gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={!activeDomain || busy}
            onClick={generateDkim}
          >
            <KeyRound />
            Generate DKIM
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!activeDomain || busy}
            onClick={() => records.refetch()}
          >
            <RefreshCw className={busy ? "animate-spin" : ""} />
            Re-plan
          </Button>
          <Button
            size="sm"
            disabled={!activeDomain || !activeProvider || busy}
            onClick={applyRecords}
          >
            <Send />
            Apply records
          </Button>
        </div>
      </div>

      <div className="mt-6 flex flex-wrap gap-8 border-y border-border py-5">
        <div>
          <div className="eyebrow">RECORDS</div>
          <div className="mt-2 text-2xl">{(records.data ?? []).length}</div>
        </div>
        <div className="border-l border-border pl-8">
          <div className="eyebrow">PUBLISHED / MANUAL</div>
          <div className="mt-2 text-2xl text-primary">{published}</div>
        </div>
        <div className="border-l border-border pl-8">
          <div className="eyebrow">PROVIDERS</div>
          <div className="mt-2 text-2xl">{(providers.data ?? []).length}</div>
        </div>
      </div>

      <div className="no-scrollbar mt-5 overflow-x-auto">
        <table className="data-table">
          <thead>
            <tr>
              <th>Purpose</th>
              <th>Type</th>
              <th>Name</th>
              <th>Value</th>
              <th>TTL</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {(records.data ?? []).map((r: DnsRecord) => (
              <tr key={r.id}>
                <td className="capitalize text-muted-foreground">{r.purpose || "—"}</td>
                <td className="font-mono text-[10px]">{r.type}</td>
                <td className="break-all font-mono text-[10px]">{r.name}</td>
                <td className="max-w-[340px] break-all text-muted-foreground">{r.value}</td>
                <td className="font-mono text-[10px] text-muted-foreground">{r.ttl}</td>
                <td>
                  <Status value={statusLabel(r.status)} />
                </td>
                <td>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    title="Copy value"
                    aria-label={`Copy ${r.name}`}
                    onClick={() => copy(r.value)}
                  >
                    <Copy className="size-3.5" />
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {activeDomain && records.data?.length === 0 && (
          <div className="py-16 text-center text-sm text-muted-foreground">
            <Check className="mx-auto mb-3 size-5 text-primary" />
            No records yet — add a server to this domain, then re-plan.
          </div>
        )}
        {!activeDomain && (
          <div className="py-16 text-center text-sm text-muted-foreground">
            Create a domain to generate DNS records.
          </div>
        )}
      </div>
    </div>
  );
}

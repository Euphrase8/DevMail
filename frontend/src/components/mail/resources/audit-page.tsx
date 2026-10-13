import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
export function AuditPage() {
  const [action, setAction] = useState("");
  const [page, setPage] = useState(1);
  const records = useQuery({
    queryKey: ["audit-page", action, page],
    queryFn: () => api.auditPage(action, page),
  });
  const exportPage = () => {
    const csv = [
      ["Time", "Actor", "Action", "Resource", "IP"],
      ...(records.data?.data ?? []).map((r) => [
        r.createdAt,
        r.actorEmail,
        r.action,
        r.resourceType + ":" + r.resourceId,
        r.ip,
      ]),
    ]
      .map((row) => row.map((cell) => `"${cell.replaceAll('"', '""')}"`).join(","))
      .join("\n");
    const url = URL.createObjectURL(new Blob([csv], { type: "text/csv" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = "devmail-audit-page.csv";
    a.click();
    URL.revokeObjectURL(url);
  };
  return (
    <div className="space-y-6">
      <h1 className="page-title">Audit log</h1>
      <label>
        Action filter
        <input
          placeholder="For example auth.login"
          value={action}
          onChange={(e) => {
            setAction(e.target.value);
            setPage(1);
          }}
        />
      </label>
      {records.isError && <p role="alert">Audit log unavailable or access denied.</p>}
      <Button disabled={!records.data} onClick={exportPage}>
        Export current page
      </Button>
      <table className="data-table">
        <thead>
          <tr>
            <th>Time</th>
            <th>Actor</th>
            <th>Action</th>
            <th>Resource</th>
            <th>IP</th>
          </tr>
        </thead>
        <tbody>
          {(records.data?.data ?? []).map((r) => (
            <tr key={r.id}>
              <td>{r.createdAt}</td>
              <td>{r.actorEmail || "Anonymous"}</td>
              <td>{r.action}</td>
              <td>
                {r.resourceType} {r.resourceId}
              </td>
              <td>{r.ip}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="flex gap-4">
        <Button disabled={page === 1} onClick={() => setPage(page - 1)}>
          Previous
        </Button>
        <span>
          Page {page} · {records.data?.meta.total ?? 0} records
        </span>
        <Button
          disabled={!records.data || page * 25 >= records.data.meta.total}
          onClick={() => setPage(page + 1)}
        >
          Next
        </Button>
      </div>
    </div>
  );
}

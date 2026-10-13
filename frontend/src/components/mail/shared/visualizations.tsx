export function Status({ value }: { value: string }) {
  const kind = ["Healthy", "Active", "Verified", "Success", "Completed"].includes(value)
    ? "healthy"
    : ["Degraded", "Warning", "Missing"].includes(value)
      ? "warning"
      : ["Failed", "Suspended"].includes(value)
        ? "error"
        : "pending";
  return <span className={`status status-${kind}`}>{value}</span>;
}

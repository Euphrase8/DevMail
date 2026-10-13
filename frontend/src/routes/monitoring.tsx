import { createFileRoute } from "@tanstack/react-router";
import { MonitoringView } from "@/components/mail/monitoring/monitoring-view";
import { routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/monitoring")({
  head: () =>
    routeMeta(
      "Monitoring",
      "Mail delivery, service availability, server resources and security alerts.",
    ),
  component: MonitoringView,
});

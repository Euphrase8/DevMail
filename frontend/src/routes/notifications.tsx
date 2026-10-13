import { createFileRoute } from "@tanstack/react-router";
import { OpsPage } from "@/components/mail/resources/ops-page";
export const Route = createFileRoute("/notifications")({ component: Page });
function Page() {
  return <OpsPage kind="notifications" />;
}

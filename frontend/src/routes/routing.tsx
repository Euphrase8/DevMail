import { createFileRoute } from "@tanstack/react-router";
import { OpsPage } from "@/components/mail/resources/ops-page";
export const Route = createFileRoute("/routing")({ component: Page });
function Page() {
  return <OpsPage kind="routing-rules" />;
}

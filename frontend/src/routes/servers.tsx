import { createFileRoute } from "@tanstack/react-router";
import { ApiResourcePage } from "@/components/mail/resources/api-resource-page";
export const Route = createFileRoute("/servers")({ component: Page });
function Page() {
  return <ApiResourcePage kind="servers" />;
}

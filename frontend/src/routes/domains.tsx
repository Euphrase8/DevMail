import { createFileRoute } from "@tanstack/react-router";
import { ApiResourcePage } from "@/components/mail/resources/api-resource-page";
export const Route = createFileRoute("/domains")({ component: Page });
function Page() {
  return <ApiResourcePage kind="domains" />;
}

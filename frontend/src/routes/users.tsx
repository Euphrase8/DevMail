import { createFileRoute } from "@tanstack/react-router";
import { ApiResourcePage } from "@/components/mail/resources/api-resource-page";
export const Route = createFileRoute("/users")({ component: Page });
function Page() {
  return <ApiResourcePage kind="users" />;
}

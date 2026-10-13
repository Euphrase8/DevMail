import { createFileRoute } from "@tanstack/react-router";
import { ResourcePage } from "@/components/mail/resources/resource-page";
import { modules, routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/users")({
  head: () => routeMeta(modules["users"]!.title, modules["users"]!.description),
  component: Page,
});
function Page() {
  return <ResourcePage section="users" />;
}

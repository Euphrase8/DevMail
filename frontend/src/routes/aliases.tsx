import { createFileRoute } from "@tanstack/react-router";
import { ResourcePage } from "@/components/mail/resources/resource-page";
import { modules, routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/aliases")({
  head: () => routeMeta(modules["aliases"]!.title, modules["aliases"]!.description),
  component: Page,
});
function Page() {
  return <ResourcePage section="aliases" />;
}

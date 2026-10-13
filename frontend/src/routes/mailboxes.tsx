import { createFileRoute } from "@tanstack/react-router";
import { ResourcePage } from "@/components/mail/resources/resource-page";
import { modules, routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/mailboxes")({
  head: () => routeMeta(modules["mailboxes"]!.title, modules["mailboxes"]!.description),
  component: Page,
});
function Page() {
  return <ResourcePage section="mailboxes" />;
}

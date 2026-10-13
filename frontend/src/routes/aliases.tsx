import { createFileRoute } from "@tanstack/react-router";
import { MailboxesPage } from "@/components/mail/resources/mailboxes-page";
import { modules, routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/aliases")({
  head: () => routeMeta(modules["aliases"]!.title, modules["aliases"]!.description),
  component: Page,
});
function Page() {
  return <MailboxesPage aliases />;
}

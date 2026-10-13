import { createFileRoute } from "@tanstack/react-router";
import { MailboxesPage } from "@/components/mail/resources/mailboxes-page";
import { modules, routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/mailboxes")({
  head: () => routeMeta(modules["mailboxes"]!.title, modules["mailboxes"]!.description),
  component: Page,
});
function Page() {
  return <MailboxesPage />;
}

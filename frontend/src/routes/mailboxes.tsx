import { createFileRoute } from "@tanstack/react-router";
import { MailboxesPage } from "@/components/mail/resources/mailboxes-page";
import { routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/mailboxes")({
  head: () => routeMeta("Mailboxes", "Domain-scoped mailbox management."),
  component: Page,
});
function Page() {
  return <MailboxesPage />;
}

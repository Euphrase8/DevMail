import { createFileRoute } from "@tanstack/react-router";
import { MailboxesPage } from "@/components/mail/resources/mailboxes-page";
import { routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/aliases")({
  head: () => routeMeta("Aliases", "Domain-scoped email alias management."),
  component: Page,
});
function Page() {
  return <MailboxesPage aliases />;
}

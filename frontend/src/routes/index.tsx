import { createFileRoute } from "@tanstack/react-router";
import { Overview } from "@/components/mail/overview/overview";
import { routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/")({
  head: () =>
    routeMeta(
      "Overview",
      "DevMail: a clear view of your self-hosted mail servers, domains, delivery and operations.",
    ),
  component: Overview,
});

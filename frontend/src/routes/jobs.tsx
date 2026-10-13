import { createFileRoute } from "@tanstack/react-router";
import { JobsPage } from "@/components/mail/jobs/jobs-page";
import { routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/jobs")({
  head: () =>
    routeMeta("Jobs", "Asynchronous provisioning tasks, their attempts and their outcomes."),
  component: JobsPage,
});

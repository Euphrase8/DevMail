import { createFileRoute } from "@tanstack/react-router";
import { DnsPage } from "@/components/mail/dns/dns-page";
import { routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/dns")({
  head: () =>
    routeMeta(
      "DNS records",
      "Generate the MX, SPF, DKIM, DMARC and verification records your domains require.",
    ),
  component: DnsPage,
});

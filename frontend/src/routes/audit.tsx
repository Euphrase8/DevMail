import { createFileRoute } from "@tanstack/react-router";
import { AuditPage } from "@/components/mail/resources/audit-page";
export const Route = createFileRoute("/audit")({ component: AuditPage });

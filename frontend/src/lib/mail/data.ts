import {
  Activity,
  ArrowLeftRight,
  AtSign,
  Bell,
  BriefcaseBusiness,
  CircleHelp,
  CloudCog,
  Globe,
  Inbox,
  LayoutDashboard,
  ListChecks,
  ScrollText,
  Server,
  Settings2,
  ShieldCheck,
  Users,
  Waypoints,
  type LucideIcon,
} from "lucide-react";
export type Section =
  | "servers"
  | "domains"
  | "dns"
  | "mailboxes"
  | "aliases"
  | "routing"
  | "monitoring"
  | "jobs"
  | "audit"
  | "users"
  | "settings"
  | "webmail"
  | "backups";
export type MailRecord = { id: string; name: string; status: string; [key: string]: string };
export type Field = {
  key: string;
  label: string;
  type?: string;
  options?: string[];
  required?: boolean;
  placeholder?: string;
};
export type Module = {
  title: string;
  description: string;
  singular: string;
  columns: [string, string][];
  fields: Field[];
  action?: string;
};
export const navigation: {
  label: string;
  path: string;
  icon: LucideIcon;
  group: string;
  count?: string;
}[] = [
  { label: "Overview", path: "/", icon: LayoutDashboard, group: "WORKSPACE" },
  { label: "Servers", path: "/servers", icon: Server, group: "INFRASTRUCTURE", count: "3" },
  { label: "Domains", path: "/domains", icon: Globe, group: "INFRASTRUCTURE", count: "8" },
  { label: "DNS records", path: "/dns", icon: Waypoints, group: "INFRASTRUCTURE" },
  {
    label: "DNS providers",
    path: "/providers",
    icon: CloudCog,
    group: "INFRASTRUCTURE",
    count: "2",
  },
  { label: "Mailboxes", path: "/mailboxes", icon: Inbox, group: "MAIL MANAGEMENT" },
  { label: "Aliases", path: "/aliases", icon: AtSign, group: "MAIL MANAGEMENT" },
  { label: "Mail routing", path: "/routing", icon: ArrowLeftRight, group: "MAIL MANAGEMENT" },
  { label: "Webmail", path: "/webmail", icon: BriefcaseBusiness, group: "MAIL MANAGEMENT" },
  { label: "Monitoring", path: "/monitoring", icon: Activity, group: "OPERATIONS" },
  { label: "Notifications", path: "/notifications", icon: Bell, group: "OPERATIONS" },
  { label: "Provisioning jobs", path: "/jobs", icon: ListChecks, group: "OPERATIONS" },
  { label: "Backups", path: "/backups", icon: ShieldCheck, group: "OPERATIONS" },
  { label: "Audit log", path: "/audit", icon: ScrollText, group: "OPERATIONS" },
  { label: "Users & access", path: "/users", icon: Users, group: "ADMINISTRATION" },
  { label: "Settings", path: "/settings", icon: Settings2, group: "ADMINISTRATION" },
  { label: "Documentation & help", path: "/docs", icon: CircleHelp, group: "ADMINISTRATION" },
];
export const modules: Record<string, Module> = {
  servers: {
    title: "Mail servers",
    description: "Your infrastructure. Connected, observable, under your control.",
    singular: "server",
    action: "Register server",
    columns: [
      ["name", "Server / hostname"],
      ["location", "Location"],
      ["ip", "IP address"],
      ["domains", "Domains"],
      ["usage", "CPU / Memory"],
      ["status", "Status"],
    ],
    fields: [
      { key: "name", label: "Server name", required: true },
      { key: "hostname", label: "Mail hostname", required: true, placeholder: "mail.example.com" },
      { key: "ip", label: "Public IPv4", required: true, placeholder: "203.0.113.10" },
      { key: "ipv6", label: "IPv6 (optional)" },
      { key: "location", label: "Location", required: true },
      { key: "environment", label: "Environment", options: ["Production", "Staging"] },
      { key: "cpu", label: "CPU cores", type: "number" },
      { key: "memory", label: "Memory (GB)", type: "number" },
      { key: "disk", label: "Disk capacity (GB)", type: "number" },
      { key: "sshUser", label: "Enrollment user (optional)" },
      { key: "sshPort", label: "SSH port", type: "number" },
    ],
  },
  domains: {
    title: "Domains",
    description: "One infrastructure. Every domain, independently managed.",
    singular: "domain",
    action: "Add domain",
    columns: [
      ["name", "Domain"],
      ["server", "Mail server"],
      ["mailboxes", "Mailboxes"],
      ["storage", "Storage"],
      ["dns", "DNS readiness"],
      ["status", "Status"],
    ],
    fields: [
      { key: "name", label: "Domain name", required: true, placeholder: "example.com" },
      {
        key: "server",
        label: "Mail server",
        options: ["eu-central-01", "us-east-01", "eu-west-01"],
      },
      { key: "provider", label: "DNS provider", options: ["Manual", "Cloudflare"] },
      { key: "quota", label: "Domain quota (GB)", type: "number" },
      { key: "limit", label: "Mailbox limit", type: "number" },
      {
        key: "catchall",
        label: "Catch-all",
        options: ["Disabled", "Enabled — increased spam risk"],
      },
    ],
  },
  dns: {
    title: "DNS records",
    description: "Every record in place. Every prerequisite visible.",
    singular: "DNS record",
    action: "Add record",
    columns: [
      ["name", "Record name"],
      ["type", "Type"],
      ["value", "Expected value"],
      ["observed", "Observed"],
      ["status", "Status"],
    ],
    fields: [
      { key: "name", label: "Record name", required: true },
      { key: "type", label: "Record type", options: ["MX", "TXT", "A", "AAAA", "CNAME", "PTR"] },
      { key: "value", label: "Expected value", required: true },
      { key: "ttl", label: "TTL (seconds)", type: "number" },
    ],
  },
  mailboxes: {
    title: "Mailboxes",
    description: "A home for every conversation. Secure by default.",
    singular: "mailbox",
    action: "Create mailbox",
    columns: [
      ["name", "Email address"],
      ["displayName", "Display name"],
      ["server", "Mail server"],
      ["storage", "Storage / quota"],
      ["lastLogin", "Last login"],
      ["status", "Status"],
    ],
    fields: [
      { key: "name", label: "Email address", type: "email", required: true },
      { key: "displayName", label: "Display name", required: true },
      { key: "password", label: "Initial password", type: "password", required: true },
      { key: "quota", label: "Quota (GB)", type: "number", required: true },
      {
        key: "server",
        label: "Mail server",
        options: ["eu-central-01", "us-east-01", "eu-west-01"],
      },
      { key: "forward", label: "Forwarding address (optional)", type: "email" },
      { key: "retain", label: "Retain local copy", options: ["Yes", "No"] },
    ],
  },
  aliases: {
    title: "Aliases",
    description: "The right address. The right destination.",
    singular: "alias",
    action: "Create alias",
    columns: [
      ["name", "Alias address"],
      ["destination", "Destinations"],
      ["domain", "Domain"],
      ["status", "Status"],
    ],
    fields: [
      { key: "name", label: "Alias address", type: "email", required: true },
      {
        key: "destination",
        label: "Destination addresses",
        required: true,
        placeholder: "alex@example.com, sam@example.com",
      },
      { key: "domain", label: "Domain", options: ["acme.studio", "orbit.design", "getnimbus.io"] },
    ],
  },
  routing: {
    title: "Mail routing",
    description: "Keep mail moving, exactly where it belongs.",
    singular: "routing rule",
    action: "Add rule",
    columns: [
      ["name", "Rule"],
      ["match", "Match"],
      ["destination", "Destination"],
      ["priority", "Priority"],
      ["status", "Status"],
    ],
    fields: [
      { key: "name", label: "Rule name", required: true },
      { key: "match", label: "Match pattern", required: true },
      { key: "destination", label: "Destination", required: true },
      { key: "priority", label: "Priority", type: "number" },
      {
        key: "type",
        label: "Rule type",
        options: ["Forwarding", "Transport", "Relay", "Catch-all"],
      },
    ],
  },
  jobs: {
    title: "Provisioning jobs",
    description: "Every change has a preflight, a progress trail, and an outcome.",
    singular: "job",
    columns: [
      ["name", "Operation"],
      ["target", "Resource"],
      ["progress", "Progress"],
      ["started", "Started"],
      ["status", "Status"],
    ],
    fields: [],
  },
  audit: {
    title: "Audit log",
    description: "A transparent history of every administrative action.",
    singular: "event",
    columns: [
      ["name", "Action"],
      ["actor", "Actor"],
      ["target", "Resource"],
      ["started", "Timestamp"],
      ["status", "Outcome"],
    ],
    fields: [],
  },
  users: {
    title: "Users & access",
    description: "The right permissions. No more, no less.",
    singular: "user",
    action: "Invite user",
    columns: [
      ["name", "User"],
      ["email", "Email"],
      ["role", "Role"],
      ["scope", "Resource scope"],
      ["lastLogin", "Last active"],
      ["status", "Status"],
    ],
    fields: [
      { key: "name", label: "Full name", required: true },
      { key: "email", label: "Email address", type: "email", required: true },
      {
        key: "role",
        label: "Role",
        options: [
          "Super Administrator",
          "Server Administrator",
          "Domain Administrator",
          "Read-only Operator",
        ],
      },
      {
        key: "scope",
        label: "Resource scope",
        options: ["All resources", "eu-central-01", "acme.studio", "orbit.design"],
      },
    ],
  },
  backups: {
    title: "Backups & recovery",
    description: "Protect your mail. Know your recovery path.",
    singular: "backup policy",
    action: "Create policy",
    columns: [
      ["name", "Backup"],
      ["target", "Server"],
      ["size", "Size"],
      ["schedule", "Schedule"],
      ["started", "Last run"],
      ["status", "Status"],
    ],
    fields: [
      { key: "name", label: "Policy name", required: true },
      { key: "target", label: "Server", options: ["eu-central-01", "us-east-01", "eu-west-01"] },
      {
        key: "schedule",
        label: "Schedule",
        options: ["Daily at 02:00 UTC", "Every 6 hours", "Weekly on Sunday"],
      },
      { key: "retention", label: "Retention (days)", type: "number" },
      {
        key: "destination",
        label: "Destination",
        options: ["S3-compatible storage", "Local volume"],
      },
    ],
  },
};
const rec = (
  id: string,
  name: string,
  status: string,
  extra: Record<string, string>,
): MailRecord => ({ id, name, status, ...extra });
export function statusKind(status: string) {
  return ["Healthy", "Active", "Verified", "Success", "Completed"].includes(status)
    ? "healthy"
    : ["Degraded", "Warning", "Missing"].includes(status)
      ? "warning"
      : ["Failed", "Suspended"].includes(status)
        ? "error"
        : "pending";
}
export const routeMeta = (title: string, description: string) => ({
  meta: [
    { title: `${title} · DevMail — Mail infrastructure` },
    { name: "description", content: description },
    { property: "og:title", content: `${title} · DevMail` },
    { property: "og:description", content: description },
    { property: "og:type", content: "website" },
    { name: "twitter:card", content: "summary_large_image" },
  ],
});

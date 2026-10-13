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

export type MailRecord = { id: string; name: string; status: string; [key: string]: string };

export const navigation: {
  label: string;
  path: string;
  icon: LucideIcon;
  group: string;
  count?: string;
}[] = [
  { label: "Overview", path: "/", icon: LayoutDashboard, group: "WORKSPACE" },
  { label: "Servers", path: "/servers", icon: Server, group: "INFRASTRUCTURE" },
  { label: "Domains", path: "/domains", icon: Globe, group: "INFRASTRUCTURE" },
  { label: "DNS records", path: "/dns", icon: Waypoints, group: "INFRASTRUCTURE" },
  { label: "DNS providers", path: "/providers", icon: CloudCog, group: "INFRASTRUCTURE" },
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

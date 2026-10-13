import { createFileRoute } from '@tanstack/react-router';
import { SettingsPage } from '@/components/mail/settings/settings-page';
import { routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/settings')({head:()=>routeMeta('Settings', 'Manage your DevMail workspace, security, mail engine, DNS providers and integrations.'),component:SettingsPage});

import { createFileRoute } from '@tanstack/react-router';
import { MonitoringPage } from '@/components/mail/monitoring/monitoring-page';
import { routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/monitoring')({head:()=>routeMeta('Monitoring', 'Mail delivery, service availability, server resources and security alerts.'),component:MonitoringPage});

import { createFileRoute } from '@tanstack/react-router';
import { NotificationsPage } from '@/components/mail/notifications/notifications-page';
import { routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/notifications')({head:()=>routeMeta('Notifications','Choose what operational alerts reach you and how. Delivery requires a connected backend.'),component:NotificationsPage});
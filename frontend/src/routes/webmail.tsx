import { createFileRoute } from '@tanstack/react-router';
import { WebmailPage } from '@/components/mail/webmail/webmail-page';
import { routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/webmail')({head:()=>routeMeta('Webmail', 'Secure webmail access for your self-hosted mailboxes.'),component:WebmailPage});

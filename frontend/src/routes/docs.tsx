import { createFileRoute } from '@tanstack/react-router';
import { DocsPage } from '@/components/mail/docs/docs-page';
import { routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/docs')({head:()=>routeMeta('Documentation','Architecture, operations, DNS, security and delivery documentation for DevMail.'),component:DocsPage});
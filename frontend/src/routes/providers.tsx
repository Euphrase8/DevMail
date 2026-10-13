import { createFileRoute } from '@tanstack/react-router';
import { ProvidersPage } from '@/components/mail/providers/providers-page';
import { routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/providers')({head:()=>routeMeta('DNS providers','Connect DNS providers or manage records manually for your self-hosted mail domains.'),component:ProvidersPage});
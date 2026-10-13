import { createFileRoute } from '@tanstack/react-router';
import { ResourcePage } from '@/components/mail/resources/resource-page';
import { modules, routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/domains')({head:()=>routeMeta(modules['domains']!.title, modules['domains']!.description), component: Page});
function Page(){return <ResourcePage section="domains"/>}

import { createFileRoute } from '@tanstack/react-router';
import { ResourcePage } from '@/components/mail/resources/resource-page';
import { modules, routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/dns')({head:()=>routeMeta(modules['dns']!.title, modules['dns']!.description), component: Page});
function Page(){return <ResourcePage section="dns"/>}

import { createFileRoute } from '@tanstack/react-router';
import { ResourcePage } from '@/components/mail/resources/resource-page';
import { modules, routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/jobs')({head:()=>routeMeta(modules['jobs']!.title, modules['jobs']!.description), component: Page});
function Page(){return <ResourcePage section="jobs"/>}

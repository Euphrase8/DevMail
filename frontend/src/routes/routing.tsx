import { createFileRoute } from '@tanstack/react-router';
import { ResourcePage } from '@/components/mail/resources/resource-page';
import { modules, routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/routing')({head:()=>routeMeta(modules['routing']!.title, modules['routing']!.description), component: Page});
function Page(){return <ResourcePage section="routing"/>}

import { createFileRoute } from '@tanstack/react-router';
import { ResourcePage } from '@/components/mail/resources/resource-page';
import { modules, routeMeta } from '@/lib/mail/data';
export const Route = createFileRoute('/servers')({head:()=>routeMeta(modules['servers']!.title, modules['servers']!.description), component: Page});
function Page(){return <ResourcePage section="servers"/>}

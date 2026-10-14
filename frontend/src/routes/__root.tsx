import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  Outlet,
  Link,
  createRootRouteWithContext,
  useRouter,
  HeadContent,
  Scripts,
  useLocation,
  useNavigate,
  type ErrorComponentProps,
} from "@tanstack/react-router";
import { useEffect, type ReactNode } from "react";

import appCss from "../styles.css?url";
import { initTheme } from "@/lib/theme";
import { MailProvider, useMail } from "@/components/mail/store";
import { MailShell, Logo } from "@/components/mail/shell/shell";

// Applied before first paint so the stored theme is on <html> during hydration
// (no flash of the wrong theme, no server/client markup mismatch).
const THEME_BOOTSTRAP = `(function(){try{var k='devmail-theme';var t=localStorage.getItem(k);if(t!=='light'&&t!=='dark'){t=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}var e=document.documentElement;e.classList.toggle('dark',t==='dark');e.style.colorScheme=t;}catch(e){}})();`;

// SessionGate keeps anonymous visitors on the auth screen. Until the session is
// known it shows a neutral splash instead of the workspace, so a signed-out
// visit never flashes the dashboard before landing on sign-in. When the API is
// unreachable the workspace renders its honest offline state.
function SessionGate({ children }: { children: ReactNode }) {
  const { session, sessionKnown, backend } = useMail();
  const navigate = useNavigate();

  useEffect(() => {
    if (backend === "online" && sessionKnown && !session) {
      navigate({ to: "/login", replace: true });
    }
  }, [backend, sessionKnown, session, navigate]);

  const settled = backend === "offline" || (sessionKnown && !!session);
  if (!settled) return <AuthSplash />;
  return <>{children}</>;
}

function AuthSplash() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="flex flex-col items-center gap-4">
        <Logo />
        <p className="font-mono text-[10px] uppercase tracking-wide text-muted-foreground">
          Checking your session…
        </p>
      </div>
    </div>
  );
}

function NotFoundComponent() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="max-w-md text-center">
        <h1 className="text-7xl font-bold text-foreground">404</h1>
        <h2 className="mt-4 text-xl font-semibold text-foreground">Page not found</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          The page you're looking for doesn't exist or has been moved.
        </p>
        <div className="mt-6">
          <Link
            to="/"
            className="inline-flex items-center justify-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
          >
            Go home
          </Link>
        </div>
      </div>
    </div>
  );
}

function ErrorComponent({ error, reset }: ErrorComponentProps) {
  console.error(error);
  const router = useRouter();

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="max-w-md text-center">
        <h1 className="text-xl font-semibold tracking-tight text-foreground">
          This page didn't load
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Something went wrong on our end. You can try refreshing or head back home.
        </p>
        <div className="mt-6 flex flex-wrap justify-center gap-2">
          <button
            onClick={() => {
              router.invalidate();
              reset();
            }}
            className="inline-flex items-center justify-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
          >
            Try again
          </button>
          <a
            href="/"
            className="inline-flex items-center justify-center rounded-md border border-input bg-background px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-accent"
          >
            Go home
          </a>
        </div>
      </div>
    </div>
  );
}

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  head: () => ({
    meta: [
      { charSet: "utf-8" },
      { name: "viewport", content: "width=device-width, initial-scale=1" },
      { title: "DevMail — Mail infrastructure" },
      {
        name: "description",
        content: "Your mail. Your infrastructure. A self-hosted mail management workspace.",
      },
      { property: "og:title", content: "DevMail — Mail infrastructure" },
      { property: "og:description", content: "Your mail. Your infrastructure." },
      { property: "og:type", content: "website" },
      { name: "twitter:card", content: "summary_large_image" },
    ],
    links: [
      {
        rel: "stylesheet",
        href: appCss,
      },
      { rel: "icon", type: "image/svg+xml", href: "/brand/devmail-art-coral.svg" },
      { rel: "icon", href: "/favicon.ico", type: "image/x-icon" },
      { rel: "preconnect", href: "https://fonts.googleapis.com" },
      { rel: "preconnect", href: "https://fonts.gstatic.com", crossOrigin: "anonymous" },
      {
        rel: "stylesheet",
        href: "https://fonts.googleapis.com/css2?family=DM+Sans:wght@400;450;500;550;600;700&family=IBM+Plex+Mono:wght@400;500&display=swap",
      },
    ],
  }),
  shellComponent: RootShell,
  component: RootComponent,
  notFoundComponent: NotFoundComponent,
  errorComponent: ErrorComponent,
});

function RootShell({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        <script dangerouslySetInnerHTML={{ __html: THEME_BOOTSTRAP }} />
        <HeadContent />
      </head>
      <body>
        {children}
        <Scripts />
      </body>
    </html>
  );
}

function RootComponent() {
  const { queryClient } = Route.useRouteContext();
  const { pathname } = useLocation();

  useEffect(() => initTheme(), []);

  return (
    <QueryClientProvider client={queryClient}>
      <MailProvider>
        {pathname === "/login" ? (
          <Outlet />
        ) : (
          <SessionGate>
            <MailShell>
              <Outlet />
            </MailShell>
          </SessionGate>
        )}
      </MailProvider>
    </QueryClientProvider>
  );
}

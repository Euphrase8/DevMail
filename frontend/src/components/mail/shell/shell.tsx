import { Link, useLocation } from "@tanstack/react-router";
import { useEffect, useState, type ReactNode } from "react";
import {
  ArrowUpRight,
  Bell,
  ChevronDown,
  ChevronsUpDown,
  CircleHelp,
  Menu,
  Search,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/theme-toggle";
import { Logo } from "@/components/brand";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { navigation } from "@/lib/mail/data";
import { useMail } from "../store";

export function MailShell({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const { notice, notify, data, backend, signOut, session, settings } = useMail();
  const [mobile, setMobile] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [panel, setPanel] = useState("");

  useEffect(() => {
    setMobile(false);
    setPanel("");
  }, [pathname]);
  useEffect(() => {
    if (!notice) return;
    const id = setTimeout(() => notify(""), 5000);
    return () => clearTimeout(id);
  }, [notice, notify]);
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setSearchOpen((v) => !v);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  return (
    <div className="min-h-screen bg-background">
      <header className="app-header sticky top-0 z-40 grid grid-cols-[minmax(0,1fr)_auto] items-center border-b border-border bg-background/80 px-3 backdrop-blur-md sm:px-5 md:px-6">
        <div className="flex min-w-0 items-center gap-2 sm:gap-7">
          <Button
            size="icon"
            variant="ghost"
            className="md:hidden"
            aria-label="Toggle navigation"
            onClick={() => setMobile(!mobile)}
          >
            <Menu />
          </Button>
          <Link to="/" aria-label="DevMail overview" className="inline-flex shrink-0">
            <Logo />
          </Link>
          <div className="hidden h-5 border-l border-border md:block" />
          <Button
            variant="ghost"
            size="sm"
            className="hidden gap-3 text-muted-foreground md:flex"
            onClick={() => setPanel(panel === "workspace" ? "" : "workspace")}
          >
            <span className="grid h-5 w-5 place-items-center rounded-sm border border-border text-[10px]">
              A
            </span>
            {settings["name"] || "DevMail"}
            <ChevronsUpDown className="size-3" />
          </Button>
        </div>
        <div className="flex shrink-0 items-center gap-1.5 md:gap-2">
          <span className="mr-1 hidden items-center gap-2 font-mono text-[10px] text-muted-foreground lg:flex">
            <span className="live-dot" />
            {backend === "online" ? "API connected" : "Not connected"}
          </span>
          <ThemeToggle />
          <Button
            aria-label="Search workspace"
            title="Search workspace (⌘K)"
            variant="ghost"
            size="icon"
            onClick={() => setSearchOpen(true)}
          >
            <Search className="size-4" />
          </Button>
          <Button
            aria-label="Notifications"
            title="Notifications"
            variant="ghost"
            size="icon"
            onClick={() => setPanel(panel === "notifications" ? "" : "notifications")}
            className="relative"
          >
            <Bell className="size-4" />
          </Button>
          <div className="h-5 border-l border-border" />
          <Button
            aria-label="Account menu"
            variant="ghost"
            size="sm"
            className="gap-2 px-1"
            onClick={() => setPanel(panel === "account" ? "" : "account")}
          >
            <span className="grid h-7 w-7 place-items-center rounded-full bg-secondary font-mono text-[10px]">
              {session?.name?.slice(0, 2).toUpperCase() || "DM"}
            </span>
            <ChevronDown className="size-3 text-muted-foreground" />
          </Button>
        </div>
      </header>

      {mobile && (
        <div
          className="fixed inset-0 top-16 z-30 bg-overlay md:hidden"
          onClick={() => setMobile(false)}
        />
      )}

      <aside
        className={`sidebar fixed flex-col border-r border-border ${mobile ? "flex" : "hidden md:flex"}`}
      >
        <nav className="small-scroll flex-1 overflow-y-auto py-2">
          {navigation.map((n, i) => (
            <div key={n.path}>
              {i === 0 && <div className="eyebrow mb-2 px-6 text-[9px]">WORKSPACE</div>}
              {i > 0 && n.group !== navigation[i - 1]?.group && (
                <div className="eyebrow mb-1.5 mt-2 px-6 text-[9px]">{n.group}</div>
              )}
              <Link
                to={n.path}
                className="nav-link mx-3"
                activeOptions={{ exact: true }}
                activeProps={{ "data-status": "active" } as Record<string, string>}
              >
                <n.icon className="size-3.5" />
                <span className="flex-1">{n.label}</span>
                {n.count && (
                  <span className="font-mono text-[10px] opacity-60">
                    {n.path === "/servers"
                      ? (data["servers"] ?? []).length
                      : n.path === "/domains"
                        ? (data["domains"] ?? []).length
                        : ""}
                  </span>
                )}
              </Link>
            </div>
          ))}
        </nav>
        <div className="border-t border-border px-5 py-2.5">
          <Button
            variant="ghost"
            size="sm"
            asChild
            className="mb-2 h-6 px-0 text-xs text-muted-foreground"
          >
            <Link to="/docs">
              <CircleHelp className="size-3.5" />
              Help & documentation
              <ArrowUpRight className="size-3" />
            </Link>
          </Button>
          <div className="flex justify-between font-mono text-[9px] text-muted-foreground">
            <span>DEVMAIL v1.0.0</span>
            <span className="text-primary">SELF-HOSTED</span>
          </div>
        </div>
      </aside>

      <main className="app-main">
        {children}
        <footer className="mt-10 flex flex-wrap items-center justify-between gap-3 border-t border-border pt-5 font-mono text-[9px] text-muted-foreground">
          <span>YOUR MAIL. YOUR INFRASTRUCTURE.</span>
          <span>
            {backend === "online" ? "API CONNECTED" : "NOT CONNECTED"}{" "}
            <span className="mx-2">/</span> UTC
          </span>
        </footer>
      </main>

      {notice && (
        <div
          role="status"
          className="page-enter fixed bottom-6 right-6 z-[60] flex max-w-[calc(100%-48px)] items-center gap-5 rounded-md border border-border bg-popover px-5 py-4 text-xs shadow-lg"
        >
          <span>{notice}</span>
          <Button
            size="icon"
            variant="ghost"
            className="h-6 w-6"
            aria-label="Dismiss notification"
            onClick={() => notify("")}
          >
            <X />
          </Button>
        </div>
      )}

      <Dialog open={searchOpen} onOpenChange={setSearchOpen}>
        <DialogContent>
          <DialogTitle className="flex items-center gap-2">
            <Search className="size-4" />
            Search workspace
          </DialogTitle>
          <DialogDescription>Find resources and workspace pages.</DialogDescription>
          <input
            autoFocus
            aria-label="Search pages and resources"
            placeholder="Search servers, domains, mailboxes…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <div className="no-scrollbar max-h-80 overflow-auto">
            {navigation
              .filter((n) => n.label.toLowerCase().includes(query.toLowerCase()))
              .map((n) => (
                <Link
                  to={n.path}
                  onClick={() => setSearchOpen(false)}
                  key={n.path}
                  className="flex items-center gap-3 border-b border-border py-3 text-sm hover:text-primary"
                >
                  <n.icon className="size-4" />
                  {n.label}
                  <ArrowUpRight className="ml-auto size-3" />
                </Link>
              ))}
            {query &&
              Object.entries(data).flatMap(([section, rows]) =>
                rows
                  .filter((r) => r.name.toLowerCase().includes(query.toLowerCase()))
                  .slice(0, 3)
                  .map((r) => (
                    <Link
                      to={`/${section}` as string}
                      onClick={() => setSearchOpen(false)}
                      key={r.id}
                      className="flex justify-between border-b border-border py-3 text-xs hover:text-primary"
                    >
                      <span>{r.name}</span>
                      <span className="text-muted-foreground">{section}</span>
                    </Link>
                  )),
              )}
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={!!panel} onOpenChange={() => setPanel("")}>
        <DialogContent>
          <DialogTitle>
            {panel === "notifications"
              ? "Notifications"
              : panel === "account"
                ? "Your account"
                : panel === "workspace"
                  ? settings["name"] || "DevMail"
                  : "Help & documentation"}
          </DialogTitle>
          <DialogDescription>
            {panel === "help"
              ? "Help topics cover the mail workflow end to end."
              : backend === "online"
                ? "Connected to the DevMail API."
                : "API offline · live workspace data unavailable"}
          </DialogDescription>
          {panel === "notifications" ? (
            <>
              <p className="text-sm">
                Review delivery events in Notifications and health checks in Monitoring.
              </p>
              <Button asChild>
                <Link to="/monitoring">Open monitoring</Link>
              </Button>
            </>
          ) : panel === "account" ? (
            <>
              <p className="text-sm">
                {session ? session.name || session.email : "Not signed in"}
                <span className="text-muted-foreground">
                  · {session ? session.role.replace(/_/g, " ") : "No session"}
                </span>
              </p>
              <Button variant="outline" asChild>
                <Link to="/settings">Account & security</Link>
              </Button>
              <Button
                variant="ghost"
                onClick={() => {
                  void signOut();
                  notify("Signed out.");
                }}
              >
                Sign out
              </Button>
            </>
          ) : panel === "workspace" ? (
            <>
              <p className="text-sm">
                {backend === "online"
                  ? `${(data["servers"] ?? []).length} servers · ${(data["domains"] ?? []).length} domains · Managed workspace`
                  : "3 servers · 8 domains · Acme workspace"}
              </p>
              <Button asChild>
                <Link to="/settings">Workspace settings</Link>
              </Button>
            </>
          ) : (
            <>
              <p className="text-sm leading-7 text-muted-foreground">
                Mailcow manages mail delivery. Ownership verification, DNS validation, server
                enrollment, mail-engine provisioning, and real delivery checks must be connected
                before production use.
              </p>
              <Button asChild variant="outline">
                <a href="https://docs.mailcow.email/" target="_blank" rel="noreferrer">
                  Mailcow documentation
                  <ArrowUpRight />
                </a>
              </Button>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

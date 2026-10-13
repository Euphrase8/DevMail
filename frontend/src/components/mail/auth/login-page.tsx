import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeftRight,
  ArrowRight,
  ChevronLeft,
  Globe,
  Inbox,
  Server,
  ShieldCheck,
  Waypoints,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Logo } from "../shell/shell";
import { api, ApiError } from "@/lib/api";

type Mode = "login" | "bootstrap" | "mfa" | "reset";

const nodes: { label: string; icon: LucideIcon; x: number; y: number }[] = [
  { label: "Mail flow", icon: ArrowLeftRight, x: 50, y: 9 },
  { label: "Domains", icon: Globe, x: 85, y: 29 },
  { label: "DNS", icon: Waypoints, x: 85, y: 71 },
  { label: "Security", icon: ShieldCheck, x: 50, y: 91 },
  { label: "Mailboxes", icon: Inbox, x: 15, y: 71 },
  { label: "Servers", icon: Server, x: 15, y: 29 },
];

const heading: Record<Mode, string> = {
  login: "Welcome back.",
  bootstrap: "Make it yours.",
  mfa: "One more checkpoint.",
  reset: "A fresh start.",
};

const subheading: Record<Mode, string> = {
  login: "Sign in to your mail infrastructure.",
  bootstrap: "Create your first administrator.",
  mfa: "Enter your six-digit authenticator code.",
  reset: "Request a secure password reset.",
};

export function LoginPage() {
  const [mode, setMode] = useState<Mode>("login");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const status = useQuery({
    queryKey: ["auth", "status"],
    queryFn: api.authStatus,
    retry: false,
    staleTime: 30_000,
  });

  useEffect(() => {
    if (status.data && !status.data.hasUsers) setMode("bootstrap");
  }, [status.data]);

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setMessage("");
    setError("");
    if (mode === "reset") {
      setMessage(
        "If an account exists, a connected authentication service would send reset instructions. No email was sent.",
      );
      return;
    }

    const form = e.currentTarget;
    const fd = new FormData(form);
    const email = String(fd.get("email") ?? "").trim();
    const password = String(fd.get("password") ?? "");
    const name = String(fd.get("name") ?? "").trim();

    setBusy(true);
    try {
      if (mode === "bootstrap") {
        await api.setup({ email, name, password });
        queryClient.setQueryData(["session"], await api.login({ email, password }));
      } else {
        queryClient.setQueryData(
          ["session"],
          await api.login({ email, password, code: String(fd.get("code") ?? "") }),
        );
      }
      queryClient.invalidateQueries({ queryKey: ["session"] });
      queryClient.invalidateQueries({ queryKey: ["health"] });
      navigate({ to: "/" });
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Could not reach the API. Is the backend running?",
      );
    } finally {
      setBusy(false);
    }
  }

  function switchMode(next: Mode) {
    setMode(next);
    setMessage("");
    setError("");
  }

  return (
    <div className="grid min-h-screen bg-background lg:grid-cols-2">
      <div className="flex flex-col">
        <header className="flex h-20 items-center justify-between px-6 md:px-10">
          <Link to="/" aria-label="DevMail overview">
            <Logo />
          </Link>
          <Button variant="ghost" size="sm" asChild>
            <Link to="/">
              <ChevronLeft />
              Return to workspace
            </Link>
          </Button>
        </header>

        <main className="flex flex-1 items-center justify-center px-6 pb-16">
          <div className="page-enter w-full max-w-sm">
            <div className="mb-6 font-mono text-[10px] uppercase tracking-wide text-primary">
              YOUR MAIL. YOUR INFRASTRUCTURE.
            </div>
            <h1 className="page-title">{heading[mode]}</h1>
            <p className="mt-4 text-sm leading-6 text-muted-foreground">{subheading[mode]}</p>

            <form className="mt-9 space-y-5" onSubmit={handleSubmit}>
              {mode === "bootstrap" && (
                <label className="block">
                  Administrator name
                  <input
                    type="text"
                    name="name"
                    required
                    autoComplete="name"
                    placeholder="Ops Admin"
                  />
                </label>
              )}
              {mode !== "mfa" && (
                <label className="block">
                  Email address
                  <input
                    type="email"
                    name="email"
                    required
                    autoCapitalize="none"
                    autoCorrect="off"
                    placeholder="you@example.com"
                    autoComplete="username"
                  />
                </label>
              )}
              {!["reset", "mfa"].includes(mode) && (
                <label className="block">
                  Password
                  <input
                    type="password"
                    name="password"
                    required
                    minLength={mode === "bootstrap" ? 12 : 8}
                    autoComplete={mode === "bootstrap" ? "new-password" : "current-password"}
                  />
                </label>
              )}
              {mode === "login" && (
                <label className="block">
                  Authenticator or recovery code (if enabled)
                  <input name="code" autoComplete="one-time-code" />
                </label>
              )}
              <Button className="w-full" type="submit" disabled={busy}>
                {mode === "reset"
                  ? "Request reset"
                  : mode === "mfa"
                    ? "Verify code"
                    : mode === "bootstrap"
                      ? "Create administrator"
                      : "Sign in"}
                <ArrowRight />
              </Button>

              {error && (
                <p
                  role="alert"
                  className="border-y border-[oklch(0.58_0.18_30)] py-4 text-xs leading-6 text-warning"
                >
                  {error}
                </p>
              )}

              {message && (
                <p
                  role="status"
                  className="border-y border-border py-4 text-xs leading-6 text-warning"
                >
                  {message}
                </p>
              )}
            </form>

            <div className="mt-8 flex flex-wrap gap-2 border-t border-border pt-5">
              {(["login", "bootstrap"] as Mode[])
                .filter((m) => m !== mode)
                .map((m) => (
                  <Button
                    key={m}
                    variant="ghost"
                    size="sm"
                    onClick={() => switchMode(m)}
                    className="px-0 text-xs text-muted-foreground"
                  >
                    {m === "login" ? "Sign in" : "Administrator setup"}
                  </Button>
                ))}
            </div>

            <p className="mt-8 flex gap-2 text-xs text-muted-foreground">
              <ShieldCheck className="size-4 text-primary" />
              {status.isLoading
                ? "Checking service availability…"
                : status.error
                  ? "API unavailable. Start the backend (go run ./cmd/api) to connect."
                  : "Connected to the DevMail API"}
            </p>
          </div>
        </main>
      </div>

      <aside className="relative hidden overflow-hidden border-l border-border bg-[oklch(0.155_0_0)] p-12 text-[oklch(0.97_0_0)] lg:flex lg:flex-col">
        <div aria-hidden className="topology pointer-events-none absolute inset-0 opacity-20" />
        <div
          aria-hidden
          className="pointer-events-none absolute left-1/2 top-1/2 h-[70%] w-[70%] -translate-x-1/2 -translate-y-1/2 rounded-full bg-[oklch(0.68_0.17_33)] opacity-[0.12] blur-3xl"
        />

        <div className="relative z-10">
          <div className="font-mono text-[10px] uppercase tracking-wide text-[oklch(0.72_0_0)]">
            Connected infrastructure
          </div>
          <h2 className="mt-5 max-w-sm text-4xl font-medium leading-[1.15]">
            Many parts.
            <br />
            One seamless system.
          </h2>
        </div>

        <div className="relative z-10 flex flex-1 items-center justify-center py-8">
          <div className="relative aspect-square h-full max-h-[360px] w-auto max-w-full">
            <div
              aria-hidden
              className="absolute left-1/2 top-1/2 h-[88%] w-[88%] -translate-x-1/2 -translate-y-1/2 rounded-full border border-white/10"
            />
            <div
              aria-hidden
              className="absolute left-1/2 top-1/2 h-[62%] w-[62%] -translate-x-1/2 -translate-y-1/2 rounded-full border border-dashed border-white/15 motion-safe:animate-[spin_44s_linear_infinite]"
            />

            <svg
              viewBox="0 0 100 100"
              preserveAspectRatio="none"
              className="absolute inset-0 h-full w-full"
              aria-hidden
            >
              {nodes.map((n) => (
                <line
                  key={n.label}
                  x1="50"
                  y1="50"
                  x2={n.x}
                  y2={n.y}
                  stroke="rgb(255 255 255 / 0.14)"
                  strokeWidth="1"
                  vectorEffect="non-scaling-stroke"
                />
              ))}
              {nodes.map((n, i) => (
                <circle key={`p-${n.label}`} r="1.5" fill="oklch(0.7 0.17 33)">
                  <animateMotion
                    path={`M50,50 L${n.x},${n.y}`}
                    dur="2.8s"
                    begin={`${i * 0.45}s`}
                    repeatCount="indefinite"
                  />
                </circle>
              ))}
            </svg>

            <div className="absolute left-1/2 top-1/2 grid h-14 w-14 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full border border-white/15 bg-white/5 backdrop-blur">
              <svg
                width="22"
                height="24"
                viewBox="0 0 25 27"
                aria-hidden
                className="text-[oklch(0.7_0.17_33)]"
              >
                <path
                  d="M2 7 12.5 1 23 7 12.5 13ZM2 13 12.5 19 23 13M2 19 12.5 25 23 19"
                  stroke="currentColor"
                  strokeWidth="2"
                  fill="none"
                />
              </svg>
            </div>

            {nodes.map(({ label, icon: Icon, x, y }) => (
              <div
                key={label}
                className="absolute flex w-24 -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-1.5"
                style={{ left: `${x}%`, top: `${y}%` }}
              >
                <span className="grid h-9 w-9 place-items-center rounded-full border border-white/15 bg-white/5 backdrop-blur">
                  <Icon className="size-4 text-[oklch(0.8_0.11_35)]" />
                </span>
                <span className="font-mono text-[9px] uppercase tracking-wide text-white/70">
                  {label}
                </span>
              </div>
            ))}
          </div>
        </div>

        <div className="relative z-10 flex flex-wrap gap-2">
          <span className="rounded-full border border-white/15 bg-white/5 px-3 py-1.5 font-mono text-[10px] tracking-wide">
            SELF-HOSTED MAIL MANAGEMENT
          </span>
          <span className="rounded-full border border-[oklch(0.68_0.17_33)]/40 bg-[oklch(0.68_0.17_33)]/10 px-3 py-1.5 font-mono text-[10px] tracking-wide text-[oklch(0.8_0.11_35)]">
            ENCRYPTED
          </span>
        </div>
      </aside>
    </div>
  );
}

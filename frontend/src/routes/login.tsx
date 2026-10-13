import { createFileRoute } from "@tanstack/react-router";
import { LoginPage } from "@/components/mail/auth/login-page";
import { routeMeta } from "@/lib/mail/data";
export const Route = createFileRoute("/login")({
  head: () =>
    routeMeta("Sign in", "Sign in to DevMail, your self-hosted mail infrastructure control panel."),
  component: LoginPage,
});

import { Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toggleTheme } from "@/lib/theme";

/**
 * Theme switch shared by the workspace shell and the authentication screen.
 *
 * The visible icon is chosen entirely in CSS from the `.dark` class on <html>,
 * so the control renders identically on the server and client (no hydration
 * flash) while still reflecting changes made anywhere in the app.
 */
export function ThemeToggle({ className }: { className?: string }) {
  return (
    <Button
      type="button"
      aria-label="Toggle color theme"
      title="Toggle color theme"
      variant="ghost"
      size="icon"
      className={className}
      onClick={() => toggleTheme()}
    >
      <Moon className="size-4 dark:hidden" aria-hidden />
      <Sun className="hidden size-4 dark:block" aria-hidden />
    </Button>
  );
}

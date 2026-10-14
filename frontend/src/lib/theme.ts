import { useSyncExternalStore } from "react";

/**
 * Theme store.
 *
 * A single, module-level source of truth for the light/dark theme so that every
 * surface — the workspace shell and the authentication screen — stays in sync.
 * The choice is persisted to localStorage, mirrored onto <html class="dark">,
 * and broadcast across tabs. An inline bootstrap script in the document head
 * applies the stored theme before first paint to avoid a flash of the wrong
 * theme during hydration.
 */
export type Theme = "light" | "dark";

const STORAGE_KEY = "devmail-theme";
const listeners = new Set<() => void>();
let current: Theme = "light";

function isTheme(value: unknown): value is Theme {
  return value === "light" || value === "dark";
}

function systemTheme(): Theme {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return "light";
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function storedTheme(): Theme | null {
  if (typeof window === "undefined") return null;
  try {
    const value = window.localStorage.getItem(STORAGE_KEY);
    return isTheme(value) ? value : null;
  } catch {
    return null;
  }
}

function paint(theme: Theme) {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  root.classList.toggle("dark", theme === "dark");
  root.style.colorScheme = theme;
}

function emit() {
  listeners.forEach((listener) => listener());
}

/** Current theme as held by the store. */
export function getTheme(): Theme {
  return current;
}

/** Persist and apply a specific theme across every surface. */
export function setTheme(theme: Theme) {
  current = theme;
  try {
    window.localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    /* storage unavailable — keep the in-memory theme for this session */
  }
  paint(theme);
  emit();
}

/** Flip between light and dark, reading the live document state as truth. */
export function toggleTheme() {
  const active =
    typeof document !== "undefined" && document.documentElement.classList.contains("dark");
  setTheme(active ? "light" : "dark");
}

export function subscribeTheme(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/**
 * Reconcile the store with storage/system preferences and keep it in sync across
 * tabs and system changes. Returns a cleanup function.
 */
export function initTheme(): () => void {
  current = storedTheme() ?? systemTheme();
  paint(current);
  emit();

  if (typeof window === "undefined") return () => {};

  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const onStorage = (event: StorageEvent) => {
    if (event.key !== STORAGE_KEY) return;
    current = isTheme(event.newValue) ? event.newValue : systemTheme();
    paint(current);
    emit();
  };
  const onSystemChange = () => {
    if (storedTheme()) return;
    current = systemTheme();
    paint(current);
    emit();
  };

  window.addEventListener("storage", onStorage);
  media.addEventListener("change", onSystemChange);
  return () => {
    window.removeEventListener("storage", onStorage);
    media.removeEventListener("change", onSystemChange);
  };
}

/** React binding for the shared theme store. */
export function useTheme(): {
  theme: Theme;
  setTheme: typeof setTheme;
  toggle: typeof toggleTheme;
} {
  const theme = useSyncExternalStore(subscribeTheme, getTheme, () => "light" as Theme);
  return { theme, setTheme, toggle: toggleTheme };
}

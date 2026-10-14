import { cn } from "@/lib/utils";

/** The supplied logo, with transparent artwork and theme-aware lettering. */
export function Logo({ className }: { className?: string }) {
  return (
    <span
      role="img"
      aria-label="DevMail"
      className={cn("inline-block w-28 shrink-0 align-middle sm:w-40", className)}
    >
      <img
        src="/brand/devmail-logo-light.svg"
        alt=""
        aria-hidden="true"
        width="800"
        height="176"
        className="block h-auto w-full dark:hidden"
      />
      <img
        src="/brand/devmail-logo-dark.svg"
        alt=""
        aria-hidden="true"
        width="800"
        height="176"
        className="hidden h-auto w-full dark:block"
      />
    </span>
  );
}

export function BrandMark({ className }: { className?: string }) {
  return (
    <img
      src="/brand/devmail-mark.svg"
      alt="DevMail"
      width="238"
      height="176"
      className={cn("h-auto w-14", className)}
    />
  );
}

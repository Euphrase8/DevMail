import type { ForwardedRef, MutableRefObject } from "react";

let lastTarget: Element | null = null;
let lastX = 0;
let lastY = 0;
let installed = false;

function ensureListener() {
  if (installed || typeof window === "undefined") return;
  installed = true;
  window.addEventListener(
    "pointerdown",
    (event) => {
      lastX = event.clientX;
      lastY = event.clientY;
      const target = event.target;
      lastTarget =
        target instanceof Element
          ? (target.closest("button, a, [role='button'], [data-dialog-origin]") ?? target)
          : null;
    },
    { capture: true, passive: true },
  );
}

export function getDialogOrigin(): { x: number; y: number } | null {
  if (typeof window === "undefined") return null;
  ensureListener();

  const target =
    (lastTarget && lastTarget.isConnected ? lastTarget : null) ??
    (document.activeElement instanceof Element ? document.activeElement : null);

  if (target && target !== document.body && target !== document.documentElement) {
    const rect = target.getBoundingClientRect();
    if (rect.width || rect.height) {
      return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
    }
  }

  if (lastX || lastY) return { x: lastX, y: lastY };
  return null;
}

export function assignRef<T>(ref: ForwardedRef<T>, value: T | null) {
  if (typeof ref === "function") ref(value);
  else if (ref) (ref as MutableRefObject<T | null>).current = value;
}

import { displays, Window, type Display } from "nsh/desktop";
import type { Bounds } from "nsh";

/** The menu bar icon's frame from the last primary click on it. */
let trayBounds: Bounds | null = null;

export function rememberTrayBounds(bounds: Bounds | null): void {
  if (bounds) trayBounds = bounds;
}

function displayFor(bounds: Bounds | null): Display | undefined {
  const all = displays();
  if (bounds) {
    const x = bounds.origin.x + bounds.size.width / 2;
    const y = bounds.origin.y + bounds.size.height / 2;
    const hit = all.find(
      (display) =>
        x >= display.bounds.origin.x &&
        x <= display.bounds.origin.x + display.bounds.size.width &&
        y >= display.bounds.origin.y &&
        y <= display.bounds.origin.y + display.bounds.size.height,
    );
    if (hit) return hit;
  }
  return all.find((display) => display.primary) ?? all[0];
}

/** Places the main window under the menu bar icon, right edges aligned,
 * clamped to the display's usable area. */
export function placeMainWindow(): void {
  const win = Window.get("main");
  if (!win) return;
  const size = win.state()?.bounds.size ?? { width: 360, height: 520 };
  const anchor = trayBounds;
  const display = displayFor(anchor);
  if (!display) return;
  const usable = display.visibleBounds;

  const margin = 8;
  const gap = 6;
  let x = usable.origin.x + usable.size.width - size.width - margin;
  let y = usable.origin.y + gap;
  if (anchor) {
    x = anchor.origin.x + anchor.size.width - size.width;
    y = anchor.origin.y + anchor.size.height + gap;
  }
  x = Math.min(Math.max(x, usable.origin.x + margin), usable.origin.x + usable.size.width - size.width - margin);
  y = Math.min(Math.max(y, usable.origin.y + gap), usable.origin.y + usable.size.height - size.height - margin);
  win.setBounds({ origin: { x: Math.round(x), y: Math.round(y) }, animated: false });
}

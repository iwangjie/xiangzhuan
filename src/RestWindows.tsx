import { effect, signal, View, WebView, type WebViewRef } from "nsh";
import { displays, Window, type Display } from "nsh/desktop";
import {
  canPostponeNow,
  canSkipNow,
  finishOutro,
  now,
  phase,
  phaseLine,
  postponeRest,
  restEnd,
  restFinishing,
  restForced,
  restInputLocked,
  restMs,
  restOpen,
  skipRest,
} from "./state";
import { SHRINE_HTML } from "./shrine-html";

// The shrine WebView is heavy (procedural textures and shaders), so load it
// once, hidden, shortly after launch; a rest then only shows it. This keeps
// the main window → rest transition smooth even on slow machines.
const prewarmed = signal(false);
setTimeout(() => prewarmed.set(true), 1400);

function showRestWindow(win: NonNullable<ReturnType<typeof Window.get>>, display: Display): void {
  const bounds = display.bounds;
  win.setBounds({
    origin: { x: bounds.origin.x, y: bounds.origin.y },
    size: { width: bounds.size.width, height: bounds.size.height },
    animated: false,
  });
  win.show();
  win.focus();
}

export function RestWindows() {
  return (
    <Each of={displays()} key={(display: Display) => display.id}>
      {(display) => (
        <Window
          id={`rest:${display().id}`}
          open={prewarmed()}
          ref={(win) => {
            win.hide();
            if (restOpen()) showRestWindow(win, display());
          }}
          title="香篆 · 休息"
          frame="none"
          defaultPosition={{ x: -12000, y: -12000 }}
          defaultSize={{ width: 1, height: 1 }}
          level="aboveDock"
          workspaces="all"
          resizable={false}
          movable={false}
          minimizable={false}
          maximizable={false}
          closable={false}
          focusable={true}
          activate={true}
          closeOnEscape={false}
          closeOnBlur={false}
          shadow={false}
          taskbar={false}
          backdrop="none"
        >
          <RestScreen display={display()} />
        </Window>
      )}
    </Each>
  );
}

/** The rest screen is the shrine page; the page draws its own controls and
 * reports them back through `window.nsh.postMessage`. */
function RestScreen(props: { display: Display }) {
  let view: WebViewRef | null = null;

  const post = () => {
    if (!view) return;
    const total = Math.max(1, restMs());
    const left = restLeft();
    view.postMessage(
      JSON.stringify({
        type: "state",
        remainingMs: Math.max(0, left),
        totalMs: total,
        progress: Math.min(1, Math.max(0, 1 - left / total)),
        canPostpone: canPostponeNow(),
        canSkip: canSkipNow(),
        forced: restForced() || restInputLocked(),
        locked: restInputLocked(),
        outro: restFinishing(),
        active: phase() === "rest",
        label: phaseLine(),
        display: props.display.name,
      }),
    );
  };

  effect(() => {
    // The rest window is prewarmed hidden; showing it is a resize + show, not
    // a page load.
    const open = restOpen();
    const win = Window.get(`rest:${props.display.id}`);
    if (!win) return;
    if (open) showRestWindow(win, props.display);
    else win.hide();
  });

  effect(() => {
    // Track every field the page shows; the 250 ms tick republishes it.
    restLeft();
    canPostponeNow();
    canSkipNow();
    restForced();
    restInputLocked();
    restFinishing();
    phase();
    phaseLine();
    post();
  });

  return (
    <View style={{ flex: 1, backgroundColor: "#070504" }}>
      <WebView
        ref={(node) => {
          view = node;
        }}
        html={SHRINE_HTML}
        style={{ flex: 1 }}
        scrollable={false}
        onLoad={() => {
          view?.focus();
          post();
        }}
        onMessage={(event) => {
          switch (String(event.data)) {
            case "ready":
              post();
              break;
            case "skip":
              skipRest();
              break;
            case "postpone":
              postponeRest();
              break;
            case "done":
              finishOutro();
              break;
            default:
              // View switches stay inside the page.
              break;
          }
        }}
      />
    </View>
  );
}

function restLeft(): number {
  const at = now();
  return Math.max(0, (restEnd() ?? at) - at);
}

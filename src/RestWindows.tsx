import { cleanup, computed, effect, signal, View, WebView, type WebViewRef } from "nsh";
import { displays, Window, type Display } from "nsh/desktop";
import {
  canPostponeNow,
  canSkipNow,
  finishOutro,
  idlePaused,
  now,
  paused,
  phase,
  phaseLine,
  postponeRest,
  remainingMs,
  restEnd,
  restFinishing,
  restForced,
  restInputLocked,
  restMs,
  restOpen,
  settings,
  skipRest,
} from "./state";
import { SHRINE_HTML } from "./shrine-html";

const PREWARM_MS = 5000;
const needsRestPage = computed(() => {
  if (restOpen()) return true;
  const current = phase();
  return (current === "focus" || current === "postpone") && !paused() && !idlePaused() && remainingMs() <= PREWARM_MS;
});

// Keep the native windows handy, but give each rest its own WebView.
const windowsReady = signal(false);
setTimeout(() => windowsReady.set(true), 1400);

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
      {(display) => <RestWindow display={display()} />}
    </Each>
  );
}

function RestWindow(props: { display: Display }) {
  effect(() => {
    const open = restOpen();
    const win = Window.get(`rest:${props.display.id}`);
    if (!win) return;
    if (open) showRestWindow(win, props.display);
    else win.hide();
  });

  return (
    <Window
      id={`rest:${props.display.id}`}
      open={windowsReady() || restOpen()}
      ref={(win) => {
        win.hide();
        if (restOpen()) showRestWindow(win, props.display);
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
      <View style={{ flex: 1, backgroundColor: "#070504" }}>
        <If when={needsRestPage()}>
          <RestScreen display={props.display} />
        </If>
      </View>
    </Window>
  );
}

/** The rest screen is the shrine page; the page draws its own controls and
 * reports them back through `window.nsh.postMessage`. */
function RestScreen(props: { display: Display }) {
  let view: WebViewRef | null = null;
  const loaded = signal(false);
  cleanup(() => { view = null; });

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
        postponeMinutes: settings().postponeMinutes,
        canSkip: canSkipNow(),
        forced: restForced() || restInputLocked(),
        locked: restInputLocked(),
        outro: restFinishing(),
        active: phase() === "rest",
        visitSeed: restEnd(),
        label: phaseLine(),
        display: props.display.name,
      }),
    );
  };

  effect(() => {
    if (restOpen() && loaded()) view?.focus();
  });

  effect(() => {
    if (loaded()) post();
  });

  return (
    <WebView
      ref={(node) => {
        view = node;
      }}
      html={SHRINE_HTML}
      style={{ flex: 1 }}
      scrollable={false}
      onLoad={() => loaded.set(true)}
      onMessage={(event) => {
        if (!view) return;
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
  );
}

function restLeft(): number {
  const at = now();
  return Math.max(0, (restEnd() ?? at) - at);
}

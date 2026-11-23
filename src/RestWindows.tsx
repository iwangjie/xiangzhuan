import { cleanup, computed, effect, platform, signal, View, WebView, type WebViewRef } from "nsh";
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
  resolveRestAction,
  restActionConfirmation,
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

const windows = platform.os === "windows";
// With a base URL, nsh's WebView2 backend serves the HTML from memory.
// This lets the scene exceed NavigateToString's 2 MB limit.
const WINDOWS_SHRINE_URL = "https://shrine.xiangzhuan.invalid/";

// The shrine page is expensive to build (procedural textures, shader compile),
// so it is only mounted in the last few seconds before a rest. Before that the
// native window exists alone, which costs nothing measurable.
const PREWARM_MS = 5000;
const needsRestPage = computed(() => {
  if (restOpen()) return true;
  const current = phase();
  return (current === "focus" || current === "postpone") && !paused() && !idlePaused() && remainingMs() <= PREWARM_MS;
});

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
  if (!windows || display.primary) win.focus();
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
      display={windows ? props.display.id : undefined}
      defaultPosition={{ x: -12000, y: -12000 }}
      defaultSize={{ width: 1, height: 1 }}
      level="aboveDock"
      workspaces="all"
      resizable={false}
      movable={false}
      minimizable={false}
      maximizable={false}
      closable={false}
      closeOnPress={windows ? false : undefined}
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
  cleanup(() => {
    view = null;
  });

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
        actionConfirmation: restActionConfirmation(),
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
    if (restOpen() && loaded() && (!windows || props.display.primary)) view?.focus();
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
      baseUrl={windows ? WINDOWS_SHRINE_URL : undefined}
      style={{ flex: 1, opacity: loaded() ? 1 : 0 }}
      scrollable={false}
      onLoad={() => {
        loaded.set(true);
        if (!windows) view?.focus();
        post();
      }}
      onMessage={(event) => {
        if (!view) return;
        const message = String(event.data);
        switch (message) {
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
          default: {
            const reply = /^(confirm|cancel):(\d+)$/.exec(message);
            if (reply) resolveRestAction(Number(reply[2]), reply[1] === "confirm");
            break;
          }
        }
      }}
    />
  );
}

function restLeft(): number {
  const at = now();
  return Math.max(0, (restEnd() ?? at) - at);
}

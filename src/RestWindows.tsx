import { effect, View, WebView, type WebViewRef } from "nsh";
import { displays, Window, type Display } from "nsh/desktop";
import {
  canPostponeNow,
  canSkipNow,
  now,
  phaseLine,
  postponeRest,
  restEnd,
  restForced,
  restMs,
  restOpen,
  skipRest,
} from "./state";
import { SHRINE_HTML } from "./shrine-html";

export function RestWindows() {
  return (
    <Each of={displays()} key={(display: Display) => display.id}>
      {(display) => (
        <Window
          id={`rest:${display().id}`}
          open={restOpen()}
          ref={(win) => {
            const bounds = display().bounds;
            win.setBounds({
              origin: { x: bounds.origin.x, y: bounds.origin.y },
              size: { width: bounds.size.width, height: bounds.size.height },
              animated: false,
            });
            win.focus();
          }}
          title="香篆 · 休息"
          frame="none"
          defaultPosition={{ x: display().bounds.origin.x, y: display().bounds.origin.y }}
          defaultSize={{ width: display().bounds.size.width, height: display().bounds.size.height }}
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
        forced: restForced(),
        label: phaseLine(),
        display: props.display.name,
      }),
    );
  };

  effect(() => {
    // Track every field the page shows; the 250 ms tick republishes it.
    restLeft();
    canPostponeNow();
    canSkipNow();
    restForced();
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

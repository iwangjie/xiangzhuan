import { batch, computed, signal, Storage } from "nsh";
import { displays, Window } from "nsh/desktop";

export type Phase = "idle" | "focus" | "rest" | "postpone";

export interface Settings {
  workMinutes: number;
  restSeconds: number;
  postponeMinutes: number;
  allowSkip: boolean;
  maxDailySkips: number;
  startAtLogin: boolean;
  /** Keep the app out of the Dock (menu bar only). */
  hideDockIcon: boolean;
  /** Keep the rest screen frontmost and swallow keyboard and mouse input. */
  lockRest: boolean;
}

export interface Stats {
  date: string;
  focusMinutes: number;
  rests: number;
  skips: number;
}

export const DEFAULT_SETTINGS: Settings = {
  workMinutes: 25,
  restSeconds: 90,
  postponeMinutes: 5,
  allowSkip: true,
  maxDailySkips: 10,
  startAtLogin: false,
  hideDockIcon: true,
  lockRest: true,
};

const SETTINGS_KEY = "xiangzhuan.settings.v1";
const STATS_KEY = "xiangzhuan.stats.v1";
const REST_ACTIVE_KEY = "restActive";
const REST_LOCK_KEY = "restLock";

/** The native dylib polls these two keys. `restActive` marks a rest screen;
 * when `restLock` is "1" it keeps that screen frontmost and swallows input.
 * Cleared on launch so a crash cannot leave the desktop locked. */
function publishRestBridge(active: boolean): void {
  void Storage.setItem(REST_ACTIVE_KEY, active ? "1" : "0").catch(() => {});
  void Storage.setItem(REST_LOCK_KEY, active && settings().lockRest ? "1" : "0").catch(() => {});
}

export const settings = signal<Settings>({ ...DEFAULT_SETTINGS });
export const settingsLoaded = signal(false);
export const stats = signal<Stats>({ date: today(), focusMinutes: 0, rests: 0, skips: 0 });

export const phase = signal<Phase>("idle");
export const now = signal(Date.now());
export const focusEnd = signal<number | null>(null);
export const restEnd = signal<number | null>(null);
export const restOpen = signal(false);
export const paused = signal(false);
let pausedAt: number | null = null;
const idlePausedAt = signal<number | null>(null);
export const idlePaused = computed(() => idlePausedAt() !== null);
let focusStartedAt = 0;
let lastHandledReturnAt = 0;
export const restForced = signal(false);
export const forceNext = signal(false);
export const postpones = signal(0);
export const loginError = signal<string | null>(null);

/** What the next rest returns to, once it is over. */
let restReturn: "focus" | "idle" = "focus";
/** Guards the delayed close of the rest windows against a new rest. */
let restGeneration = 0;
/** The page is playing its tea-pour outro; the host stops the rest when it
 * reports `done` or the fallback timer below fires. */
export const restFinishing = signal(false);
let finishTimer: number | null = null;

const OUTRO_MS = 12000;
const FOCUS_IDLE_MS = 5 * 60_000;

export interface InputActivity {
  idleSince: number | null;
  lastAway: { since: number; until: number } | null;
}

function clearFinishTimer(): void {
  if (finishTimer != null) {
    clearTimeout(finishTimer);
    finishTimer = null;
  }
  restFinishing.set(false);
}

export function today(): string {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export const workMs = computed(() => Math.max(1, settings().workMinutes) * 60_000);
export const restMs = computed(() => Math.max(1, settings().restSeconds) * 1000);
export const postponeMs = computed(() => Math.max(1, settings().postponeMinutes) * 60_000);

/** How much is left in the current phase, in milliseconds. */
export const remainingMs = computed(() => {
  const p = phase();
  const at = now();
  if (p === "rest") return restEnd() == null ? restMs() : Math.max(0, restEnd()! - at);
  if (p === "focus" || p === "postpone") {
    const end = focusEnd();
    if (end == null) return workMs();
    const frozen = paused() ? pausedAt : idlePausedAt();
    return Math.max(0, end - (frozen ?? at));
  }
  return workMs();
});

/** The length of the current phase, in milliseconds. */
export const totalMs = computed(() => {
  const p = phase();
  if (p === "rest") return restMs();
  if (p === "postpone") return postponeMs();
  return workMs();
});

/** How much of the current phase is left, from 1 to 0. */
export const remainingPart = computed(() => {
  const total = totalMs();
  return total <= 0 ? 0 : Math.min(1, Math.max(0, remainingMs() / total));
});

/** The native rest lock is engaged for this rest. `restForced` is the
 * per-cycle rule; this is the user's blanket lock. */
export const restInputLocked = computed(() => phase() === "rest" && settings().lockRest);
export const skipEnabled = computed(() => settings().allowSkip && !settings().lockRest);

export const phaseLabel = computed(() => {
  switch (phase()) {
    case "focus":
      return paused() ? "暂停" : idlePaused() ? "空闲暂停" : "专注";
    case "rest":
      return "休息";
    case "postpone":
      return "延后";
    default:
      return "待燃";
  }
});

export const phaseLine = computed(() => {
  switch (phase()) {
    case "focus":
      if (paused()) return "已暂停，点继续。";
      return idlePaused() ? "空闲已暂停，操作键鼠后继续。" : "香正在燃，别急。";
    case "rest":
      return restForced() || restInputLocked() ? "这炷香，要燃尽。" : "把眼睛从屏幕上移开。";
    case "postpone":
      return "再处理一小会儿。";
    default:
      return "准备好时，点一炷香。";
  }
});

export const canSkipNow = computed(
  () =>
    phase() === "rest" &&
    !restFinishing() &&
    skipEnabled() &&
    !restForced() &&
    stats().skips < Math.max(0, settings().maxDailySkips),
);
export const canPostponeNow = computed(
  () => phase() === "rest" && !restFinishing() && !restInputLocked() && !restForced(),
);

export function formatClock(ms: number): string {
  const total = Math.max(0, Math.ceil(ms / 1000));
  const m = Math.floor(total / 60);
  const s = total % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export function ensureToday(): void {
  const date = today();
  if (stats().date !== date) {
    stats.set({ date, focusMinutes: 0, rests: 0, skips: 0 });
  }
}

function addFocusMinutes(minutes: number): void {
  ensureToday();
  stats.update((s) => ({ ...s, focusMinutes: s.focusMinutes + Math.max(1, Math.round(minutes)) }));
}

function addRest(): void {
  ensureToday();
  stats.update((s) => ({ ...s, rests: s.rests + 1 }));
}

function addSkip(): void {
  ensureToday();
  stats.update((s) => ({ ...s, skips: s.skips + 1 }));
}

export function loadSettings(): void {
  publishRestBridge(false);
  void Storage.getItem(SETTINGS_KEY)
    .then((raw) => {
      if (!raw) return;
      const next = { ...DEFAULT_SETTINGS, ...(JSON.parse(raw) as Partial<Settings>) };
      settings.set(next);
    })
    .catch(() => {})
    .finally(() => settingsLoaded.set(true));

  void Storage.getItem(STATS_KEY)
    .then((raw) => {
      if (!raw) return;
      const next = JSON.parse(raw) as Partial<Stats>;
      const date = today();
      if (next.date === date) stats.set({ ...stats(), ...next });
      else stats.set({ date, focusMinutes: 0, rests: 0, skips: 0 });
    })
    .catch(() => {});
}

export function saveSettings(): void {
  if (!settingsLoaded()) return;
  void Storage.setItem(SETTINGS_KEY, JSON.stringify(settings())).catch(() => {});
  // Native `dock_hide.dylib` watches this key and switches the activation
  // policy live; absent or "1" keeps the app in the menu bar only.
  void Storage.setItem("hideDockIcon", settings().hideDockIcon ? "1" : "0").catch(() => {});
}

export function saveStats(): void {
  void Storage.setItem(STATS_KEY, JSON.stringify(stats())).catch(() => {});
}

export function updateSettings(patch: Partial<Settings>): void {
  settings.update((s) => ({ ...s, ...patch }));
  if (phase() === "rest") publishRestBridge(true);
}

export function setAllowSkip(allowSkip: boolean): void {
  updateSettings(allowSkip ? { allowSkip, lockRest: false } : { allowSkip });
}

export function startFocus(): void {
  if (phase() === "rest") return;
  batch(() => startFocusCountdown(Date.now()));
}

function startFocusCountdown(at: number, left = workMs()): void {
  focusStartedAt = at;
  lastHandledReturnAt = at;
  focusEnd.set(at + left);
  now.set(at);
  paused.set(false);
  pausedAt = null;
  idlePausedAt.set(null);
  phase.set("focus");
}

export function pauseFocus(): void {
  if (phase() !== "focus" || paused()) return;
  batch(() => {
    const at = Date.now();
    pausedAt = idlePausedAt() ?? at;
    paused.set(true);
    idlePausedAt.set(null);
    now.set(at);
  });
}

export function resumeFocus(): void {
  if (phase() !== "focus" || !paused()) return;
  const end = focusEnd();
  const from = pausedAt ?? Date.now();
  const left = end == null ? workMs() : Math.max(0, end - from);
  batch(() => startFocusCountdown(Date.now(), left));
}

export function togglePause(): void {
  if (paused()) resumeFocus();
  else pauseFocus();
}

/** Changing the focus length mid-session keeps the elapsed fraction. */
export function setWorkMinutes(minutes: number): void {
  const next = Math.max(1, Math.round(minutes));
  const end = focusEnd();
  if (phase() === "focus" && end != null) {
    const oldTotal = workMs();
    const at = (paused() ? pausedAt : idlePausedAt()) ?? Date.now();
    const left = Math.max(0, end - at);
    const part = oldTotal > 0 ? left / oldTotal : 1;
    focusEnd.set(at + next * 60_000 * part);
  }
  updateSettings({ workMinutes: next });
}

export function startRest(completedWork = false): void {
  if (phase() === "rest") return;
  if (completedWork) addFocusMinutes(settings().workMinutes);
  restReturn = phase() === "idle" ? "idle" : "focus";
  paused.set(false);
  pausedAt = null;
  idlePausedAt.set(null);
  restForced.set(forceNext());
  clearFinishTimer();
  restGeneration++;
  restOpen.set(true);
  publishRestBridge(true);
  focusEnd.set(null);
  restEnd.set(Date.now() + restMs());
  phase.set("rest");
  Window.get("main")?.hide();
  saveStats();
}

function closeRestWindows(): void {
  const generation = ++restGeneration;
  setTimeout(() => {
    if (generation !== restGeneration) return;
    restOpen.set(false);
    publishRestBridge(false);
  }, 420);
}

function nextWork(): void {
  if (restReturn === "idle") {
    focusEnd.set(null);
    phase.set("idle");
  } else {
    startFocusCountdown(Date.now());
  }
}

export function finishRest(): void {
  if (phase() !== "rest" || restFinishing()) return;
  // The shrine page owns the next ~7 s: the incense burns out, the teapot
  // pours a cup, the room fades to black, then it reports "done". The 12 s
  // timer only ends a rest whose page never reports back.
  restFinishing.set(true);
  restEnd.set(null);
  paused.set(false);
  pausedAt = null;
  restForced.set(false);
  postpones.set(0);
  forceNext.set(false);
  clearTimeout(finishTimer ?? undefined);
  finishTimer = setTimeout(finishOutro, OUTRO_MS);
}

/** Ends the rest after the page's outro (called by "done" or the fallback timer). */
export function finishOutro(): void {
  if (phase() !== "rest" || !restFinishing()) return;
  clearFinishTimer();
  addRest();
  closeRestWindows();
  nextWork();
  saveStats();
}

export function postponeRest(): void {
  if (!canPostponeNow()) return;
  clearFinishTimer();
  paused.set(false);
  pausedAt = null;
  const n = postpones() + 1;
  postpones.set(n);
  if (n >= 3) forceNext.set(true);
  restEnd.set(null);
  restForced.set(false);
  closeRestWindows();
  focusEnd.set(Date.now() + postponeMs());
  phase.set("postpone");
}

export function skipRest(): void {
  if (!canSkipNow()) return;
  clearFinishTimer();
  forceNext.set(true);
  paused.set(false);
  pausedAt = null;
  restEnd.set(null);
  restForced.set(false);
  addSkip();
  closeRestWindows();
  nextWork();
  saveStats();
}

export function stopCycle(): void {
  clearFinishTimer();
  paused.set(false);
  pausedAt = null;
  idlePausedAt.set(null);
  if (phase() === "rest") {
    restEnd.set(null);
    restForced.set(false);
    closeRestWindows();
  }
  phase.set("idle");
  focusEnd.set(null);
}

function applyFocusActivity(at: number, activity: InputActivity): void {
  const away = activity.lastAway;
  const end = focusEnd();
  if (end == null) return;

  if (away && away.until > lastHandledReturnAt && away.until <= at) {
    lastHandledReturnAt = away.until;
    if (away.until - away.since >= FOCUS_IDLE_MS) {
      const frozen = idlePausedAt() ?? Math.max(focusStartedAt, away.since + FOCUS_IDLE_MS);
      const left = away.until - away.since >= workMs() ? workMs() : Math.max(0, end - frozen);
      focusEnd.set(away.until + left);
      focusStartedAt = away.until;
      idlePausedAt.set(null);
    }
  }

  const since = activity.idleSince;
  const frozen = idlePausedAt();
  if (frozen != null && (since == null || at - since < FOCUS_IDLE_MS)) {
    // An input may land between the native sample and the timer tick
    // right at the threshold, before there is a completed idle interval.
    focusEnd.set(at + Math.max(0, focusEnd()! - frozen));
    idlePausedAt.set(null);
  } else if (frozen == null && since != null && at - since >= FOCUS_IDLE_MS) {
    idlePausedAt.set(Math.max(focusStartedAt, since + FOCUS_IDLE_MS));
  }
}

export function tick(at = Date.now(), activity?: InputActivity): void {
  batch(() => advanceClock(at, activity));
}

function advanceClock(at: number, activity?: InputActivity): void {
  ensureToday();
  now.set(at);
  const p = phase();
  if (p === "rest") {
    const end = restEnd();
    if (end != null && at >= end) finishRest();
    return;
  }
  if (p === "focus") {
    if (paused()) return;
    if (activity) applyFocusActivity(at, activity);
    if (idlePaused()) return;
  }
  if (p === "focus" || p === "postpone") {
    const end = focusEnd();
    if (end != null && at >= end) startRest(p === "focus");
  }
}

export function showApp(): void {
  if (phase() === "rest") {
    const screen = displays()[0];
    if (screen) Window.get(`rest:${screen.id}`)?.focus();
    return;
  }
  tick();
  const main = Window.get("main");
  main?.show();
  main?.focus();
}

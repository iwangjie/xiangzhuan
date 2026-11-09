import { expect, test } from "nsh/test";
import {
  DEFAULT_SETTINGS,
  canPostponeNow,
  canSkipNow,
  finishOutro,
  finishRest,
  focusEnd,
  forceNext,
  pauseFocus,
  phase,
  postponeRest,
  postpones,
  remainingMs,
  restEnd,
  restForced,
  restOpen,
  resumeFocus,
  setWorkMinutes,
  settings,
  skipRest,
  startFocus,
  startRest,
  stats,
  tick,
} from "./state";

function reset(): void {
  phase.set("idle");
  focusEnd.set(null);
  restEnd.set(null);
  restOpen.set(false);
  restForced.set(false);
  forceNext.set(false);
  postpones.set(0);
  settings.set({ ...DEFAULT_SETTINGS });
}

function restFromFocus(): void {
  startFocus();
  focusEnd.set(Date.now() - 1);
  tick();
  expect(phase()).toBe("rest");
}

test("a focus ends into a full rest", () => {
  reset();
  restFromFocus();
  expect(restOpen()).toBe(true);
  expect(restForced()).toBe(false);
});

test("three postponements make the next rest forced", () => {
  reset();
  restFromFocus();
  for (let i = 0; i < 3; i++) {
    expect(canPostponeNow()).toBe(true);
    postponeRest();
    expect(phase()).toBe("postpone");
    startRest();
  }
  expect(forceNext()).toBe(true);
  expect(restForced()).toBe(true);
  expect(canPostponeNow()).toBe(false);
  expect(canSkipNow()).toBe(false);
  finishRest();
  // The rest window keeps playing the burn-out/pour outro; it ends on "done".
  expect(phase()).toBe("rest");
  finishOutro();
  expect(forceNext()).toBe(false);
  expect(phase()).toBe("focus");
});

test("skipping a rest forces the next one", () => {
  reset();
  restFromFocus();
  expect(canSkipNow()).toBe(true);
  skipRest();
  expect(forceNext()).toBe(true);
  expect(phase()).toBe("focus");
  startRest();
  expect(restForced()).toBe(true);
});

test("a rest with skipping off can only be postponed", () => {
  reset();
  settings.update((s) => ({ ...s, allowSkip: false }));
  restFromFocus();
  expect(canSkipNow()).toBe(false);
  skipRest();
  expect(phase()).toBe("rest");
  expect(canPostponeNow()).toBe(true);
  postponeRest();
  expect(phase()).toBe("postpone");
});

test("pausing freezes focus and resuming gives the time back", () => {
  reset();
  startFocus();
  const end = focusEnd();
  expect(end).not.toBe(null);
  pauseFocus();
  const frozen = remainingMs();
  tick();
  expect(remainingMs()).toBe(frozen);
  expect(focusEnd()).toBe(end);
  resumeFocus();
  // The paused duration (0 ms inside one test) is returned; never lost.
  expect(focusEnd()).toBeGreaterThanOrEqual(end!);
});

test("changing the focus length mid-session rescales the remainder", () => {
  reset();
  startFocus();
  const before = remainingMs();
  setWorkMinutes(50);
  expect(settings().workMinutes).toBe(50);
  expect(remainingMs()).toBeGreaterThan(before * 1.8);
});

test("the daily skip cap stops skipping", () => {
  reset();
  settings.update((s) => ({ ...s, maxDailySkips: 1 }));
  stats.update((s) => ({ ...s, skips: 1 }));
  restFromFocus();
  expect(canSkipNow()).toBe(false);
  skipRest();
  expect(phase()).toBe("rest");
});

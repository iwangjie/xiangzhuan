import { expect, test } from "nsh/test";
import {
  DEFAULT_SETTINGS,
  canPostponeNow,
  canSkipNow,
  finishOutro,
  finishRest,
  focusEnd,
  forceNext,
  idlePaused,
  pauseFocus,
  phase,
  postponeRest,
  postpones,
  remainingMs,
  resolveRestAction,
  restActionConfirmation,
  restEnd,
  restForced,
  restFinishing,
  restInputLocked,
  restOpen,
  resumeFocus,
  setAllowSkip,
  setWorkMinutes,
  settings,
  skipEnabled,
  skipRest,
  startFocus,
  startRest,
  stats,
  stopCycle,
  tick,
  today,
} from "./state";

function reset(): void {
  stopCycle();
  focusEnd.set(null);
  restEnd.set(null);
  restOpen.set(false);
  restForced.set(false);
  forceNext.set(false);
  postpones.set(0);
  settings.set({ ...DEFAULT_SETTINGS, lockRest: false });
  stats.set({ date: today(), focusMinutes: 0, rests: 0, skips: 0 });
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

test("the default rest locks out skipping and postponing", () => {
  reset();
  settings.update((s) => ({ ...s, lockRest: true }));
  restFromFocus();
  expect(restInputLocked()).toBe(true);
  expect(canSkipNow()).toBe(false);
  expect(canPostponeNow()).toBe(false);
  skipRest();
  expect(phase()).toBe("rest");
  postponeRest();
  expect(phase()).toBe("rest");
  expect(restOpen()).toBe(true);
});

test("enabling skipping unlocks rest actions from the default settings", () => {
  reset();
  settings.set({ ...DEFAULT_SETTINGS });
  expect(skipEnabled()).toBe(false);
  setAllowSkip(true);
  expect(skipEnabled()).toBe(true);
  expect(settings().lockRest).toBe(false);
  restFromFocus();
  expect(canSkipNow()).toBe(true);
  expect(canPostponeNow()).toBe(true);
  setAllowSkip(false);
  expect(canSkipNow()).toBe(false);
  expect(canPostponeNow()).toBe(true);
});

test("only the third postponement asks for confirmation and forces the next rest", () => {
  reset();
  restFromFocus();
  for (let i = 0; i < 2; i++) {
    expect(canPostponeNow()).toBe(true);
    postponeRest();
    expect(restActionConfirmation()).toBe(null);
    expect(phase()).toBe("postpone");
    startRest();
  }
  postponeRest();
  expect(restActionConfirmation()?.action).toBe("postpone");
  expect(postpones()).toBe(2);
  expect(forceNext()).toBe(false);
  expect(phase()).toBe("rest");
  resolveRestAction(restActionConfirmation()!.id, false);
  expect(restActionConfirmation()).toBe(null);
  expect(postpones()).toBe(2);
  expect(phase()).toBe("rest");
  postponeRest();
  resolveRestAction(restActionConfirmation()!.id, true);
  expect(phase()).toBe("postpone");
  expect(forceNext()).toBe(true);
  startRest();
  expect(restForced()).toBe(true);
  expect(canPostponeNow()).toBe(false);
  expect(canSkipNow()).toBe(false);
  finishRest();
  // The rest window keeps playing the burn-out/pour outro; it ends on "done".
  expect(phase()).toBe("rest");
  expect(canPostponeNow()).toBe(false);
  expect(canSkipNow()).toBe(false);
  finishOutro();
  expect(forceNext()).toBe(false);
  expect(phase()).toBe("focus");
  startRest();
  expect(canSkipNow()).toBe(true);
});

test("a skip requires confirmation and forces the next rest only once", () => {
  reset();
  restFromFocus();
  skipRest();
  const first = restActionConfirmation()!;
  expect(first.action).toBe("skip");
  expect(phase()).toBe("rest");
  expect(stats().skips).toBe(0);
  expect(forceNext()).toBe(false);
  skipRest();
  expect(restActionConfirmation()?.id).toBe(first.id);
  resolveRestAction(first.id, false);
  expect(phase()).toBe("rest");
  expect(stats().skips).toBe(0);
  skipRest();
  const second = restActionConfirmation()!;
  resolveRestAction(first.id, true);
  expect(restActionConfirmation()?.id).toBe(second.id);
  expect(phase()).toBe("rest");
  resolveRestAction(second.id, true);
  resolveRestAction(second.id, true);
  expect(stats().skips).toBe(1);
  expect(forceNext()).toBe(true);
  expect(phase()).toBe("focus");
  startRest();
  expect(restForced()).toBe(true);
});

test("rest expiry dismisses the pending confirmation even when the user confirms first", () => {
  reset();
  restFromFocus();
  skipRest();
  const confirmation = restActionConfirmation()!;
  restEnd.set(Date.now() - 1);
  resolveRestAction(confirmation.id, true);
  expect(restActionConfirmation()).toBe(null);
  expect(phase()).toBe("rest");
  expect(restFinishing()).toBe(true);
  expect(stats().skips).toBe(0);
  expect(forceNext()).toBe(false);
  finishOutro();
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

const minute = 60_000;

test("five idle minutes freeze the timer and a short absence resumes the remainder", () => {
  reset();
  startFocus();
  const since = focusEnd()! - 23 * minute;
  tick(since + 5 * minute - 1, { idleSince: since, lastAway: null });
  expect(idlePaused()).toBe(false);
  tick(since + 5 * minute + 200, { idleSince: since, lastAway: null });
  expect(idlePaused()).toBe(true);
  expect(remainingMs()).toBe(18 * minute);
  tick(since + 12 * minute, { idleSince: since, lastAway: null });
  expect(remainingMs()).toBe(18 * minute);
  expect(phase()).toBe("focus");
  const until = since + 12 * minute;
  const activity = { idleSince: null, lastAway: { since, until } };
  tick(until, activity);
  expect(idlePaused()).toBe(false);
  expect(remainingMs()).toBe(18 * minute);
  tick(until + minute, activity);
  expect(remainingMs()).toBe(17 * minute);
});

test("an absence as long as the configured focus restarts a full round on input", () => {
  reset();
  settings.update((value) => ({ ...value, workMinutes: 45 }));
  startFocus();
  const since = focusEnd()! - 40 * minute;
  const until = since + 45 * minute;
  const focusBefore = stats().focusMinutes;
  tick(since + 5 * minute, { idleSince: since, lastAway: null });
  tick(until, { idleSince: since, lastAway: null });
  expect(remainingMs()).toBe(35 * minute);
  expect(phase()).toBe("focus");
  tick(until, { idleSince: null, lastAway: { since, until } });
  expect(remainingMs()).toBe(45 * minute);
  expect(stats().focusMinutes).toBe(focusBefore);
});

test("the recorded idle interval handles wake even if no tick saw the pause", () => {
  reset();
  startFocus();
  const since = focusEnd()! - 22 * minute;
  const until = since + 28 * minute;
  tick(until + 100, { idleSince: null, lastAway: { since, until } });
  expect(phase()).toBe("focus");
  expect(remainingMs()).toBe(25 * minute - 100);
});

test("input at the pause boundary releases a freeze even without a recorded absence", () => {
  reset();
  startFocus();
  const since = focusEnd()! - 25 * minute;
  const at = since + 5 * minute;
  tick(at, { idleSince: since, lastAway: null });
  expect(idlePaused()).toBe(true);
  tick(at + 100, { idleSince: null, lastAway: null });
  expect(idlePaused()).toBe(false);
  expect(remainingMs()).toBe(20 * minute);
});

test("idle tracking leaves manual pause and the active rest countdown alone", () => {
  reset();
  startFocus();
  pauseFocus();
  const frozen = remainingMs();
  const since = Date.now() + 1;
  const until = since + 30 * minute;
  tick(until, { idleSince: null, lastAway: { since, until } });
  expect(remainingMs()).toBe(frozen);
  expect(idlePaused()).toBe(false);
  resumeFocus();
  startRest();
  tick(restEnd()!, { idleSince: Date.now() - 30 * minute, lastAway: null });
  expect(restFinishing()).toBe(true);
});

test("the daily skip count no longer prevents an eligible skip", () => {
  reset();
  stats.update((s) => ({ ...s, skips: 24 }));
  restFromFocus();
  expect(canSkipNow()).toBe(true);
  skipRest();
  resolveRestAction(restActionConfirmation()!.id, true);
  expect(stats().skips).toBe(25);
  expect(phase()).toBe("focus");
});

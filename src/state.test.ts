import { expect, test } from "nsh/test";
import {
  DEFAULT_SETTINGS,
  canPostponeNow,
  canSkipNow,
  finishRest,
  focusEnd,
  forceNext,
  phase,
  postponeRest,
  postpones,
  restEnd,
  restForced,
  restOpen,
  settings,
  skipRest,
  startFocus,
  startRest,
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

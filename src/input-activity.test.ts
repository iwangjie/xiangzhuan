import { expect, test } from "nsh/test";
import { createWindowsInputMonitor } from "./input-activity";

test("Windows idle samples preserve the start of an absence and report the return", () => {
  let idleMs = 1000;
  const read = createWindowsInputMonitor(() => idleMs);
  expect(read(11_000)).toEqual({ idleSince: 10_000, lastAway: null });

  idleMs = 299_990;
  expect(read(310_000)).toEqual({ idleSince: 10_000, lastAway: null });

  idleMs = 500;
  expect(read(365_500)).toEqual({ idleSince: 365_000, lastAway: { since: 10_000, until: 365_000 } });
});

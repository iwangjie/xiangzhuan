import { expect, open, test, type App } from "nsh/test";

const SETTINGS = "xiangzhuan.settings.v1";

function restWindowId(app: App): string {
  return `rest:${app.window("main").state()?.display ?? "1"}`;
}

test("rest windows stay hidden through focus and only show at the rest", async () => {
  const app = await open("../App.tsx", {
    platform: "macos",
    clock: {},
    width: 440,
    height: 680,
    storage: { [SETTINGS]: JSON.stringify({ workMinutes: 25, restSeconds: 90 }) },
  });

  const main = app.window("main");
  const restId = restWindowId(app);
  expect(main.state()?.visible).toBe(true);
  // The native windows are created for reuse once the app is up, but they are
  // never shown during focus.
  await app.clock!.runFor(2000);
  expect(app.window(restId).state()?.visible ?? false).toBe(false);

  await main.pressClose();
  expect(main.state()?.visible).toBe(false);
  await app.clock!.fastForward(24 * 60_000);
  expect(app.window(restId).state()?.visible).toBe(false);

  await app.clock!.runFor(61_000);
  expect(app.window(restId).state()?.visible).toBe(true);
});

test("pausing focus freezes the countdown across long idle periods", async () => {
  const app = await open("../App.tsx", {
    platform: "macos",
    clock: {},
    width: 440,
    height: 680,
    storage: { [SETTINGS]: JSON.stringify({ workMinutes: 25, restSeconds: 90 }) },
  });

  const restId = restWindowId(app);
  await app.press("pause-focus");
  await app.window("main").pressClose();

  await app.clock!.fastForward(30 * 60_000);
  expect(app.window(restId).state()?.visible).toBe(false);
});

test("rest windows hide again once the rest finishes", async () => {
  const app = await open("../App.tsx", {
    platform: "macos",
    clock: {},
    width: 440,
    height: 680,
    storage: { [SETTINGS]: JSON.stringify({ workMinutes: 1, restSeconds: 10, lockRest: false }) },
  });

  const restId = restWindowId(app);
  await app.window("main").pressClose();

  await app.clock!.fastForward(61_000);
  expect(app.window(restId).state()?.visible).toBe(true);

  // 10 s rest + the 12 s outro fallback + the 420 ms close delay.
  await app.clock!.runFor(11_000);
  await app.clock!.runFor(13_000);
  expect(app.window(restId).state()?.visible).toBe(false);
});

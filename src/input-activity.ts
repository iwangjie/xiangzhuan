import fs from "fs/promises";
import os from "os";
import { join } from "path";
import type { InputActivity } from "./state";

let lastMtimeMs: number | null = null;
let cached: InputActivity | undefined;

export async function openWindowsInputMonitor(): Promise<() => InputActivity | undefined> {
  // The Windows release runner registers this optional native module.
  const moduleName = "xiangzhuan/windows";
  const { InputActivity } = await import(moduleName) as {
    InputActivity: { idleMilliseconds(): number | null | undefined };
  };
  return createWindowsInputMonitor(() => InputActivity.idleMilliseconds());
}

export function createWindowsInputMonitor(readIdleMs: () => number | null | undefined) {
  let inputAt: number | null = null;
  let lastAway: InputActivity["lastAway"] = null;
  return (at = Date.now()): InputActivity | undefined => {
    const idleMs = readIdleMs();
    if (idleMs == null) return undefined;
    const lastInput = at - idleMs;
    // The Win32 tick and wall clock can disagree by a few milliseconds.
    if (inputAt == null || lastInput < inputAt - 1000) {
      inputAt = lastInput;
      lastAway = null;
    } else if (lastInput > inputAt + 1000) {
      lastAway = { since: inputAt, until: lastInput };
      inputAt = lastInput;
    }
    return { idleSince: inputAt, lastAway };
  };
}

export async function readInputActivity(): Promise<InputActivity | undefined> {
  try {
    const path = join(os.homedir(), "Library", "Application Support", "com.iwangjie.xiangzhuan", "input-activity", "snapshot.json");
    const info = await fs.stat(path);
    if (lastMtimeMs !== null && info.mtimeMs === lastMtimeMs) return cached;
    const { idleSince, lastAway } = JSON.parse(await fs.readFile(path, "utf8"));
    if (idleSince !== null && !Number.isFinite(idleSince)) return undefined;
    lastMtimeMs = info.mtimeMs;
    cached = {
      idleSince,
      lastAway: Array.isArray(lastAway) && lastAway.length === 2 &&
        Number.isFinite(lastAway[0]) && Number.isFinite(lastAway[1]) && lastAway[0] <= lastAway[1]
        ? { since: lastAway[0], until: lastAway[1] }
        : null,
    };
    return cached;
  } catch {
    // The native helper may not have initialized yet (or may be absent in dev).
    lastMtimeMs = null;
    cached = undefined;
    return undefined;
  }
}

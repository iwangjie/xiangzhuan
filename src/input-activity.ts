import fs from "fs/promises";
import os from "os";
import { join } from "path";
import type { InputActivity } from "./state";

const path = join(os.homedir(), "Library", "Application Support", "com.iwangjie.xiangzhuan", "input-activity", "snapshot.json");

let lastMtimeMs: number | null = null;
let cached: InputActivity | undefined;

export async function readInputActivity(): Promise<InputActivity | undefined> {
  try {
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

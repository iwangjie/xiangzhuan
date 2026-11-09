import fs from "fs/promises";
import os from "os";
import { join } from "path";
import process from "process";
import { loginError, settings } from "./state";

const LABEL = "com.iwangjie.xiangzhuan.login";
const BUNDLE_ID = "com.iwangjie.xiangzhuan";

const PLIST = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/bin/open</string>
    <string>-b</string>
    <string>${BUNDLE_ID}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>LimitLoadToSessionType</key>
  <string>Aqua</string>
  <key>ProcessType</key>
  <string>Interactive</string>
</dict>
</plist>
`;

function plistPath(): string {
  return join(os.homedir(), "Library", "LaunchAgents", `${LABEL}.plist`);
}

async function currentUid(): Promise<string> {
  const result = await process.run("/usr/bin/id", ["-u"]);
  return result.stdout.trim();
}

export async function setLoginItem(on: boolean): Promise<boolean> {
  try {
    const path = plistPath();
    const uid = await currentUid();
    if (on) {
      await fs.mkdir(join(os.homedir(), "Library", "LaunchAgents"), { recursive: true });
      await fs.writeFile(path, PLIST);
      await process.run("/bin/launchctl", ["bootout", `gui/${uid}`, path]).catch(() => {});
      const result = await process.run("/bin/launchctl", ["bootstrap", `gui/${uid}`, path]);
      if (result.code !== 0) throw new Error(result.stderr.trim() || "launchctl bootstrap failed");
    } else {
      await process.run("/bin/launchctl", ["bootout", `gui/${uid}`, path]).catch(() => {});
      await fs.unlink(path).catch(() => {});
    }
    loginError.set(null);
    return true;
  } catch (error) {
    loginError.set(error instanceof Error ? error.message : String(error));
    return false;
  }
}

/** Writes the login item once, after an installed app starts with it on. */
export async function ensureLoginItem(): Promise<void> {
  if (process.env.NSH_APP_DIR) return;
  if (!settings().startAtLogin) return;
  if (!(await fs.exists("/Applications/香篆.app"))) return;
  await setLoginItem(true);
}

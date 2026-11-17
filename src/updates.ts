import { signal } from "nsh";
import {
  checkForPackage,
  checkForUpdate,
  downloadPackage,
  downloadUpdate,
  installAndRestart,
  restartToPackage,
} from "nsh/updater";
import { phase } from "./state";

/** The version being downloaded or installed, for the settings row. */
export const pendingVersion = signal<string | null>(null);
export const checking = signal(false);
/** 0..1 while an archive downloads. */
export const progress = signal<number | null>(null);
/** The downloaded app is being swapped in; the app restarts right after. */
export const applying = signal(false);
export const updateError = signal<string | null>(null);

let active = false;

const message = (error: unknown) => (error instanceof Error ? error.message : String(error));

let restartScheduled = false;

/** A rest owns the screen and the input lock: the restart waits until it is
 * over, so an update never cuts a rest short. */
function restartWhenFree(restart: () => Promise<void>): void {
  if (restartScheduled) return;
  restartScheduled = true;
  const attempt = () => {
    if (phase() === "rest") {
      setTimeout(attempt, 2000);
      return;
    }
    void restart().catch((error) => {
      restartScheduled = false;
      updateError.set(message(error));
      applying.set(false);
    });
  };
  attempt();
}

/** Looks for a new package first, then a new app build, and applies it
 * without asking: both paths end in a restart. */
export async function checkForUpdates(): Promise<void> {
  if (active) return;
  active = true;
  checking.set(true);
  updateError.set(null);
  try {
    const pkg = await checkForPackage();
    if (pkg) {
      pendingVersion.set(pkg.version);
      await downloadPackage(({ received, total }) => progress.set(total ? received / total : null));
      progress.set(null);
      applying.set(true);
      restartWhenFree(restartToPackage);
      return;
    }

    const app = await checkForUpdate();
    if (!app) return;
    pendingVersion.set(app.version);
    if (!app.canInstall) {
      updateError.set("新版本需要手动安装");
      return;
    }
    await downloadUpdate(({ received, total }) => progress.set(total ? received / total : null));
    progress.set(null);
    applying.set(true);
    restartWhenFree(installAndRestart);
  } catch (error) {
    updateError.set(message(error));
    applying.set(false);
  } finally {
    active = false;
    checking.set(false);
    progress.set(null);
  }
}

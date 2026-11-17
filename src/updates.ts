import { openUrl, signal } from "nsh";
import {
  checkForPackage,
  checkForUpdate,
  downloadPackage,
  downloadUpdate,
  installAndRestart,
  restartToPackage,
  type AppUpdate,
} from "nsh/updater";

/** An OTA package is downloaded and waiting for the next start. */
export const packageReady = signal(false);
/** A new app build, for a change a package cannot carry. */
export const appUpdate = signal<AppUpdate | null>(null);
export const checking = signal(false);
export const progress = signal<number | null>(null);
export const updateError = signal<string | null>(null);

let active = false;

export async function checkForUpdates(options: { quiet?: boolean } = {}): Promise<void> {
  if (active) return;
  active = true;
  checking.set(true);
  updateError.set(null);
  try {
    // The popover's code changes far more often than its native shell, so a
    // package comes first; a new app build only when there is none.
    if (await checkForPackage()) {
      await downloadPackage(({ received, total }) => progress.set(total ? received / total : null));
      packageReady.set(true);
      return;
    }
    appUpdate.set(await checkForUpdate());
  } catch (error) {
    if (!options.quiet) updateError.set(error instanceof Error ? error.message : String(error));
  } finally {
    active = false;
    checking.set(false);
    progress.set(null);
  }
}

export async function restartIntoPackage(): Promise<void> {
  try {
    await restartToPackage();
  } catch (error) {
    updateError.set(error instanceof Error ? error.message : String(error));
  }
}

export async function installAppUpdate(): Promise<void> {
  const update = appUpdate();
  if (!update) return;
  if (!update.canInstall) {
    await openUrl(update.url);
    return;
  }
  checking.set(true);
  updateError.set(null);
  try {
    await downloadUpdate(({ received, total }) => progress.set(total ? received / total : null));
    await installAndRestart();
  } catch (error) {
    updateError.set(error instanceof Error ? error.message : String(error));
  } finally {
    checking.set(false);
    progress.set(null);
  }
}

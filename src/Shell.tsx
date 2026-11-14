import { cleanup, computed, effect, keymap, lifecycle, untracked } from "nsh";
import { Menu, Tray, Window, type MenuItem } from "nsh/desktop";
import trayIcon from "../assets/tray.svg";
import { readInputActivity } from "./input-activity";
import { ensureLoginItem, setLoginItem } from "./login-item";
import {
  canPostponeNow,
  canSkipNow,
  focusEnd,
  formatClock,
  idlePaused,
  loadSettings,
  paused,
  phase,
  phaseLabel,
  postponeRest,
  remainingMs,
  restEnd,
  restFinishing,
  restForced,
  restInputLocked,
  saveSettings,
  setAllowSkip,
  settings,
  settingsLoaded,
  showApp,
  skipEnabled,
  skipRest,
  startFocus,
  startRest,
  stopCycle,
  tick,
  togglePause,
  updateSettings,
} from "./state";

const trayTime = computed(() => {
  const left = remainingMs();
  if (left < 60_000) return formatClock(left);
  return `${Math.ceil(left / 60_000)} 分钟`;
});

const trayLine = computed(() => {
  if (phase() === "idle") return "香篆 · 待燃";
  return `香篆 · ${phaseLabel()} ${trayTime()}`;
});

const statusLine = computed(() => {
  if (phase() === "idle") return "待燃";
  if (phase() === "focus" && idlePaused()) return `空闲暂停 · ${trayTime()}`;
  if (phase() === "rest" && restInputLocked()) return "锁定休息中";
  if (phase() === "rest" && restForced()) return "强制休息中";
  return `${phaseLabel()}中 · ${trayTime()}`;
});

function trayMenu(): (MenuItem | false)[] {
  const p = phase();
  const items: (MenuItem | false)[] = [
    { label: statusLine(), disabled: true },
    { type: "separator" },
    { label: p === "rest" ? "打开休息屏" : "显示香篆", onSelect: showApp },
  ];

  if (p === "rest") {
    items.push(
      canPostponeNow() && {
        label: `延后 ${settings().postponeMinutes} 分钟`,
        onSelect: postponeRest,
      },
      canSkipNow() && { label: "跳过休息", onSelect: skipRest },
      restInputLocked() && { label: "锁定休息中，无法提前结束", disabled: true },
      restForced() && { label: "强制休息中，无法提前结束", disabled: true },
    );
  } else {
    items.push(
      p === "idle"
        ? { label: "开始专注", shortcut: "mod+shift+s", onSelect: startFocus }
        : { label: "立即休息", shortcut: "mod+shift+r", onSelect: () => startRest(false) },
      p === "focus" && {
        label: paused() ? "继续专注" : "暂停专注",
        shortcut: "mod+shift+p",
        onSelect: togglePause,
      },
      p === "idle" && { label: "直接休息", onSelect: () => startRest(false) },
      p !== "idle" && { label: "结束本轮", onSelect: stopCycle },
    );
  }

  items.push(
    { type: "separator" },
    {
      label: "允许跳过休息",
      checked: skipEnabled(),
      onSelect: () => setAllowSkip(!skipEnabled()),
    },
    {
      label: "登录时启动",
      checked: settings().startAtLogin,
      onSelect: () => {
        const next = !settings().startAtLogin;
        updateSettings({ startAtLogin: next });
        void setLoginItem(next);
      },
    },
    { type: "separator" },
    { role: "quit", label: "退出香篆" },
  );

  return items;
}

function appMenu(): (MenuItem | false)[] {
  return [
    {
      label: "香篆",
      submenu: [
        { role: "about", label: "关于香篆" },
        { type: "separator" },
        { label: "显示香篆", shortcut: "mod+0", onSelect: showApp },
        { label: "开始专注", shortcut: "mod+shift+s", onSelect: startFocus },
        { label: "暂停 / 继续专注", shortcut: "mod+shift+p", onSelect: togglePause },
        { label: "立即休息", shortcut: "mod+shift+r", onSelect: () => startRest(false) },
        { type: "separator" },
        { role: "hide", label: "隐藏香篆" },
        { role: "hideOthers", label: "隐藏其他" },
        { type: "separator" },
        { role: "quit", label: "退出香篆" },
      ],
    },
    {
      label: "编辑",
      submenu: [
        { role: "undo" },
        { role: "redo" },
        { type: "separator" },
        { role: "cut" },
        { role: "copy" },
        { role: "paste" },
        { role: "selectAll" },
      ],
    },
    {
      label: "窗口",
      submenu: [
        { role: "minimize" },
        { role: "zoom" },
        { role: "toggleFullScreen" },
        { type: "separator" },
        { role: "close" },
      ],
    },
  ];
}

export function Shell() {
  loadSettings();

  effect(() => {
    Tray.set({
      icon: trayIcon,
      tooltip: trayLine(),
      onClick: showApp,
      menu: trayMenu(),
    });
  });

  effect(() => {
    Menu.setApplicationMenu(appMenu());
  });

  // 启动即进入专注；用菜单/快捷键暂停，而不是要求用户手动开始。
  let autoStarted = false;
  effect(() => {
    if (autoStarted || !settingsLoaded()) return;
    if (phase() !== "idle") return;
    autoStarted = true;
    startFocus();
  });

  keymap.bind("mod+shift+s", () => {
    if (phase() === "idle") startFocus();
  });
  keymap.bind("mod+shift+p", () => {
    if (phase() === "focus") togglePause();
  });
  keymap.bind("mod+shift+r", () => startRest(false));
  keymap.bind(
    "escape",
    () => {
      if (canSkipNow()) skipRest();
    },
    { when: "notEditing" },
  );

  let checkingInput = false;
  let disposed = false;
  cleanup(() => {
    disposed = true;
  });

  const checkActivity = () => {
    if (checkingInput || phase() !== "focus" || paused()) return;
    checkingInput = true;
    void readInputActivity()
      .then((activity) => {
        if (disposed || !activity) return;
        if (activity.idleSince !== null || activity.lastAway !== null || idlePaused()) {
          tick(Date.now(), activity);
        }
      })
      .finally(() => {
        checkingInput = false;
      });
  };

  const syncFocus = () => {
    tick();
    checkActivity();
  };

  effect(() => {
    const p = phase();
    const isPaused = paused();
    const isIdlePaused = idlePaused();
    const fEnd = focusEnd();
    const rEnd = restEnd();
    const finishing = restFinishing();
    const mainVisible = Window.get("main")?.state()?.visible ?? false;
    const appLife = lifecycle();
    void p;
    void isPaused;
    void isIdlePaused;
    void fEnd;
    void rEnd;
    void finishing;
    void mainVisible;
    void appLife;

    let timer: number | undefined;
    let cancelled = false;

    const arm = () => {
      if (cancelled) return;
      const delay = untracked(() => nextTickDelay(mainVisible));
      if (delay != null) timer = setTimeout(onTick, delay);
    };

    const onTick = () => {
      if (cancelled) return;
      untracked(syncFocus);
      arm();
    };

    arm();
    cleanup(() => {
      cancelled = true;
      if (timer !== undefined) clearTimeout(timer);
    });
  });

  effect(() => {
    if (lifecycle() === "active") {
      const wakeTimer = setTimeout(() => untracked(() => syncFocus()), 0);
      cleanup(() => clearTimeout(wakeTimer));
    }
  });

  let loginChecked = false;
  effect(() => {
    if (!settingsLoaded() || loginChecked) return;
    loginChecked = true;
    void ensureLoginItem();
  });

  effect(() => {
    settings();
    if (settingsLoaded()) saveSettings();
  });

  return null;
}

function nextTickDelay(mainVisible: boolean): number | null {
  const p = phase();
  if (p === "idle") return null;
  if (p === "focus" && paused()) return null;
  if (p === "focus" && idlePaused()) return mainVisible ? 1000 : 5000;

  if (p === "rest") {
    if (restFinishing()) return null;
    const end = restEnd();
    if (end == null) return null;
    const left = end - Date.now();
    if (left <= 0) return 16;
    return Math.min(left, ((left - 1) % 1000) + 1 + 15);
  }

  const end = focusEnd();
  if (end == null) return null;
  const left = end - Date.now();
  if (left <= 0) return 16;

  if (mainVisible || left <= 60_000) {
    return Math.min(left, ((left - 1) % 1000) + 1 + 15);
  }

  // Main window is hidden and > 60 s remain: the tray only shows whole minutes.
  const mins = Math.ceil(left / 60_000);
  const untilDrop = mins > 1 ? left - (mins - 1) * 60_000 : left - 59_999;
  return Math.min(left, Math.max(1, untilDrop) + 15);
}

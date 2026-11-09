import { cleanup, computed, effect, keymap, lifecycle, untracked } from "nsh";
import { Menu, Tray, type MenuItem } from "nsh/desktop";
import trayIcon from "../assets/tray.svg";
import { ensureLoginItem, setLoginItem } from "./login-item";
import {
  canPostponeNow,
  canSkipNow,
  formatClock,
  loadSettings,
  phase,
  phaseLabel,
  postponeRest,
  remainingMs,
  restForced,
  saveSettings,
  settings,
  settingsLoaded,
  showApp,
  skipRest,
  startFocus,
  startRest,
  stopCycle,
  tick,
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
      restForced() && { label: "强制休息中，无法提前结束", disabled: true },
    );
  } else {
    items.push(
      p === "idle"
        ? { label: "开始专注", shortcut: "mod+shift+s", onSelect: startFocus }
        : { label: "立即休息", shortcut: "mod+shift+r", onSelect: () => startRest(false) },
      p === "idle" && { label: "直接休息", onSelect: () => startRest(false) },
      p !== "idle" && { label: "结束本轮", onSelect: stopCycle },
    );
  }

  items.push(
    { type: "separator" },
    {
      label: "允许跳过休息",
      checked: settings().allowSkip,
      onSelect: () => updateSettings({ allowSkip: !settings().allowSkip }),
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

  keymap.bind("mod+shift+s", () => startFocus());
  keymap.bind("mod+shift+r", () => startRest(false));
  keymap.bind(
    "escape",
    () => {
      if (canSkipNow()) skipRest();
    },
    { when: "notEditing" },
  );

  effect(() => {
    untracked(tick);
    const timer = setInterval(() => tick(), 250);
    cleanup(() => clearInterval(timer));
  });

  effect(() => {
    if (lifecycle() === "active") untracked(tick);
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

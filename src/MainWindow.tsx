import {
  cleanup,
  createAnimatedValue,
  createMotion,
  Easing,
  effect,
  keymap,
  reduceMotion,
  Scroll,
  signal,
  Text,
  View,
  untracked,
} from "nsh";
import { Button, Switch } from "nsh/base";
import { Application, currentWindow } from "nsh/desktop";
import {
  canPostponeNow,
  canSkipNow,
  forceNext,
  formatClock,
  loginError,
  pauseFocus,
  paused,
  phase,
  phaseLabel,
  phaseLine,
  postponeRest,
  postpones,
  remainingMs,
  remainingPart,
  restForced,
  restInputLocked,
  resumeFocus,
  setAllowSkip,
  settings,
  setWorkMinutes,
  showApp,
  skipEnabled,
  skipRest,
  startFocus,
  startRest,
  stats,
  stopCycle,
  updateSettings,
} from "./state";
import { setLoginItem } from "./login-item";
import { Action, Card, Dot, KeyHint, Ring, Segmented } from "./ui";

const WORK_OPTIONS = [25, 45, 60] as const;
const REST_OPTIONS = [60, 90, 120] as const;
const POSTPONE_OPTIONS = [5, 10, 15] as const;

const TABS = [
  { id: "timer", label: "计时" },
  { id: "rhythm", label: "节奏" },
  { id: "rules", label: "规则" },
] as const;
type TabId = (typeof TABS)[number]["id"];

export function MainWindow() {
  const win = currentWindow();
  const ring = createAnimatedValue(1);
  const head = createMotion({ enter: "fadeInUp", duration: 240 });
  const body = createMotion({ enter: "fadeInUp", duration: 280, delay: 40 });
  const tab = signal<TabId>("timer");

  effect(() => {
    win.set({
      closeOnPress: false,
      title: "香篆",
      frame: "none",
      titleBar: { style: "hidden", height: 0 },
      level: "floating",
      resizable: false,
      minimizable: false,
      maximizable: false,
      closable: true,
      movable: true,
      sizeToContent: false,
      shadow: true,
      transparent: true,
    });
    placeMainWindow();
    const stopClose = win.on("closePress", () => win.hide());
    // A menu bar popover goes away when another app takes the focus. A window
    // shown without the focus (a background reopen) is left alone: only a
    // focus it actually had and lost counts.
    let hadFocus = false;
    const stopFocus = win.on("focusChange", ({ focused }) => {
      if (focused) {
        hadFocus = true;
        return;
      }
      if (!hadFocus) return;
      hadFocus = false;
      if (phase() !== "rest") win.hide();
    });
    cleanup(() => {
      stopClose();
      stopFocus();
    });
  });

  // A rest owns the screen. Hide the main window even if it mounted after
  // `startRest` ran, and never show it again when the rest ends.
  effect(() => {
    if (phase() === "rest") win.hide();
  });

  const visible = () => win.state()?.visible ?? false;
  const shownRemainingMs = () => (visible() ? remainingMs() : untracked(remainingMs));

  let shownPart = 1;
  let wasVisible = false;
  effect(() => {
    const shown = visible();
    const part = remainingPart();
    if (!shown) {
      wasVisible = false;
      return;
    }
    if (!wasVisible) {
      wasVisible = true;
      shownPart = part;
      ring.set(part);
      return;
    }
    if (part === shownPart) return;
    shownPart = part;
    if (reduceMotion()) ring.set(part);
    else ring.timing(part, { duration: 480, easing: Easing.linear });
  });

  const isRest = () => phase() === "rest";
  const isIdle = () => phase() === "idle";
  const isFocus = () => phase() === "focus";
  const isPaused = () => isFocus() && paused();

  const tabButton = (id: TabId, label: string) => (
    <Button
      onPress={() => tab.set(id)}
      class={`flex-1 items-center justify-center rounded-[8px] py-1.5 transition duration-150 ${
        tab() === id
          ? "bg-card text-foreground shadow-sm"
          : "text-muted-foreground hover:text-foreground"
      }`}
    >
      <Text class={`text-[12px] font-medium ${tab() === id ? "" : "text-muted-foreground"}`}>
        {label}
      </Text>
    </Button>
  );

  return (
    <View style={{ flex: 1, backgroundColor: "transparent" }}>
      <View class="flex-1 overflow-hidden rounded-(--radius-card) border border-border bg-background">
        <View
          desktop={{ windowDrag: true }}
          class="h-10 flex-row items-center gap-2 px-3"
        >
          <View class="size-[18px] items-center justify-center rounded-[5px] bg-foreground">
            <Text
              style={{ fontFamily: "Songti SC", fontSize: 10, lineHeight: 12 }}
              class="text-background"
            >
              香
            </Text>
          </View>
          <Text class="text-[13px] font-semibold tracking-[0.02em] text-foreground">香篆</Text>
          <View class="flex-1" />
          <View class="flex-row items-center gap-1.5">
            <Dot tone={isRest() ? "rest" : isIdle() ? "idle" : "work"} />
            <Text class="text-[10px] text-muted-foreground">{phaseLabel()}</Text>
          </View>
        </View>

        <View class="mx-3 mb-2 flex-row gap-1 rounded-(--radius) bg-muted p-1">
          {TABS.map((item) => tabButton(item.id, item.label))}
        </View>

        <Scroll
          style={{ flex: 1 }}
          scrollbar={false}
          contentStyle={{ paddingLeft: 12, paddingRight: 12, paddingBottom: 12, gap: 10 }}
        >
          <If when={tab() === "timer"}>
            <View style={{ ...head.style, gap: 10 }}>
              <Card style={{ paddingTop: 18, paddingBottom: 16, paddingLeft: 14, paddingRight: 14 }}>
                <View class="items-center gap-3">
                  <Ring progress={ring} size={152} tone="main">
                    <Text class="text-[30px] font-light leading-none tracking-[-0.03em] text-countdown tabular-nums">
                      {formatClock(shownRemainingMs())}
                    </Text>
                    <If when={isIdle() || isPaused() || idlePaused()}>
                      <Text class="mt-1 text-[10px] tracking-[0.03em] text-muted-foreground">
                        {isIdle()
                          ? `一炷香 · ${settings().workMinutes} 分钟`
                          : isPaused()
                            ? "已暂停 · 点继续"
                            : phaseLine()}
                      </Text>
                    </If>
                  </Ring>

                  <View class="w-full gap-1.5">
                    <If when={isIdle()}>
                      <Action label="开始专注" variant="primary" onPress={startFocus} testID="start-focus" class="w-full" />
                      <Action label="直接休息" variant="ghost" onPress={() => startRest(false)} class="w-full" />
                    </If>
                    <If when={isFocus()}>
                      <Action
                        label={isPaused() ? "继续专注" : "暂停专注"}
                        variant="primary"
                        onPress={isPaused() ? resumeFocus : pauseFocus}
                        testID={isPaused() ? "resume-focus" : "pause-focus"}
                        class="w-full"
                      />
                      <View class="flex-row gap-1.5">
                        <Action label="立即休息" variant="ghost" onPress={() => startRest(false)} testID="rest-now" class="flex-1" />
                        <Action label="结束本轮" variant="ghost" onPress={stopCycle} class="flex-1" />
                      </View>
                    </If>
                    <If when={isRest()}>
                      <Text class="text-center text-[11px] leading-4 text-muted-foreground">
                        {phaseLine()}
                      </Text>
                      <View class="flex-row gap-1.5">
                        <If when={canPostponeNow()}>
                          <Action
                            label={`延后 ${settings().postponeMinutes} 分钟`}
                            onPress={postponeRest}
                            class="flex-1"
                          />
                        </If>
                        <If when={canSkipNow()}>
                          <Action label="提前结束…" variant="ghost" onPress={skipRest} class="flex-1" />
                        </If>
                      </View>
                      <If when={!canPostponeNow() && !canSkipNow()}>
                        <Text class="text-center text-[11px] tracking-[0.08em] text-muted-foreground">
                          {restForced() ? "强制休息中" : restInputLocked() ? "锁定休息中" : "静候香尽"}
                        </Text>
                      </If>
                      <Action label="回到休息屏" variant="ghost" onPress={showApp} class="w-full" />
                    </If>
                    <If when={phase() === "postpone"}>
                      <Action label="立即休息" onPress={() => startRest(false)} class="w-full" />
                      <Action label="结束本轮" variant="ghost" onPress={stopCycle} class="w-full" />
                    </If>
                  </View>

                  <View class="flex-row items-center gap-1.5">
                    <KeyHint keys={keymap.label("mod+shift+p")} />
                    <Text class="text-[9px] text-faint">暂停</Text>
                    <Text class="text-[9px] text-faint">·</Text>
                    <KeyHint keys={keymap.label("mod+shift+r")} />
                    <Text class="text-[9px] text-faint">休息</Text>
                  </View>
                </View>
              </Card>

              <Card style={{ padding: 0 }}>
                <View class="flex-row">
                  <View class="flex-1 items-center gap-0.5 py-2.5">
                    <Text class="text-[13px] font-medium tabular-nums text-foreground">
                      {stats().focusMinutes}
                      <Text class="ml-0.5 text-[9px] font-normal text-muted-foreground">分钟</Text>
                    </Text>
                    <Text class="text-[9px] text-muted-foreground">今日专注</Text>
                  </View>
                  <View class="my-2 w-px bg-border/70" />
                  <View class="flex-1 items-center gap-0.5 py-2.5">
                    <Text class="text-[13px] font-medium tabular-nums text-foreground">
                      {stats().rests}
                      <Text class="ml-0.5 text-[9px] font-normal text-muted-foreground">次</Text>
                    </Text>
                    <Text class="text-[9px] text-muted-foreground">完成休息</Text>
                  </View>
                  <View class="my-2 w-px bg-border/70" />
                  <View class="flex-1 items-center gap-0.5 py-2.5">
                    <Text class="text-[13px] font-medium tabular-nums text-foreground">
                      {stats().skips}
                      <Text class="ml-0.5 text-[9px] font-normal text-muted-foreground">次</Text>
                    </Text>
                    <Text class="text-[9px] text-muted-foreground">跳过休息</Text>
                  </View>
                </View>
              </Card>
            </View>
          </If>

          <If when={tab() === "rhythm"}>
            <View style={{ ...body.style, gap: 10 }}>
              <Card style={{ padding: 0 }}>
                <View class="px-4 py-3">
                  <View class="flex-row items-center justify-between">
                    <Text class="text-[12px] font-medium text-foreground">专注</Text>
                    <Text class="text-[10px] text-muted-foreground">
                      {settings().workMinutes} 分钟 · 一炷香
                    </Text>
                  </View>
                  <View class="mt-2">
                    <Segmented
                      label="专注时长"
                      value={settings().workMinutes}
                      options={WORK_OPTIONS.map((m) => ({ value: m, label: `${m} 分` }))}
                      onChange={(workMinutes) => setWorkMinutes(workMinutes)}
                      disabled={isRest()}
                    />
                  </View>
                </View>
                <View class="h-px bg-border/70" />
                <View class="px-4 py-3">
                  <View class="flex-row items-center justify-between">
                    <Text class="text-[12px] font-medium text-foreground">休息</Text>
                    <Text class="text-[10px] text-muted-foreground">{settings().restSeconds} 秒</Text>
                  </View>
                  <View class="mt-2">
                    <Segmented
                      label="休息时长"
                      value={settings().restSeconds}
                      options={REST_OPTIONS.map((s) => ({ value: s, label: `${s} 秒` }))}
                      onChange={(restSeconds) => updateSettings({ restSeconds })}
                    />
                  </View>
                </View>
                <View class="h-px bg-border/70" />
                <View class="px-4 py-3">
                  <View class="flex-row items-center justify-between">
                    <Text class="text-[12px] font-medium text-foreground">延后</Text>
                    <Text class="text-[10px] text-muted-foreground">急事时先收起休息屏</Text>
                  </View>
                  <View class="mt-2">
                    <Segmented
                      label="延后时长"
                      value={settings().postponeMinutes}
                      options={POSTPONE_OPTIONS.map((m) => ({ value: m, label: `${m} 分` }))}
                      onChange={(postponeMinutes) => updateSettings({ postponeMinutes })}
                    />
                  </View>
                </View>
              </Card>
              <If when={!isIdle()}>
                <Text class="px-1 text-[10px] leading-4 text-muted-foreground">
                  本轮进行中，时长改动按剩余比例生效。
                </Text>
              </If>
            </View>
          </If>

          <If when={tab() === "rules"}>
            <View style={{ ...body.style, gap: 10 }}>
              <Card style={{ padding: 0 }}>
                <View class="flex-row items-center justify-between gap-3 px-4 py-3">
                  <View style={{ flex: 1, gap: 2 }}>
                    <Text class="text-[12px] font-medium text-foreground">休息时锁定输入</Text>
                    <Text class="text-[10px] leading-4 text-muted-foreground">
                      拦截键盘鼠标与切屏，只能等香燃尽
                    </Text>
                  </View>
                  <Switch
                    checked={settings().lockRest}
                    onChange={({ checked }) => updateSettings({ lockRest: checked })}
                    accessibilityLabel="休息时锁定输入"
                    trackClass="h-6 w-11 rounded-full bg-muted p-0.5 transition-colors checked:bg-primary"
                    thumbClass="size-5 rounded-full bg-background shadow-sm transition-transform checked:translate-x-5"
                  />
                </View>
                <View class="h-px bg-border/70" />
                <View class="flex-row items-center justify-between gap-3 px-4 py-3">
                  <View style={{ flex: 1, gap: 2 }}>
                    <Text class="text-[12px] font-medium text-foreground">允许跳过休息</Text>
                    <Text class="text-[10px] leading-4 text-muted-foreground">
                      {settings().lockRest ? "开启后会解除休息时的输入锁定" : "关闭后仍可延后休息"}
                    </Text>
                  </View>
                  <Switch
                    checked={skipEnabled()}
                    onChange={({ checked }) => setAllowSkip(checked)}
                    accessibilityLabel="允许跳过休息"
                    trackClass="h-6 w-11 rounded-full bg-muted p-0.5 transition-colors checked:bg-primary"
                    thumbClass="size-5 rounded-full bg-background shadow-sm transition-transform checked:translate-x-5"
                  />
                </View>
                <View class="h-px bg-border/70" />
                <View class="flex-row items-center justify-between gap-3 px-4 py-3">
                  <View style={{ flex: 1, gap: 2 }}>
                    <Text class="text-[12px] font-medium text-foreground">登录时启动</Text>
                    <Text class="text-[10px] leading-4 text-muted-foreground">
                      {loginError() ?? "登录后在菜单栏等你"}
                    </Text>
                  </View>
                  <Switch
                    checked={settings().startAtLogin}
                    onChange={({ checked }) => {
                      updateSettings({ startAtLogin: checked });
                      void setLoginItem(checked);
                    }}
                    accessibilityLabel="登录时启动"
                    trackClass="h-6 w-11 rounded-full bg-muted p-0.5 transition-colors checked:bg-primary"
                    thumbClass="size-5 rounded-full bg-background shadow-sm transition-transform checked:translate-x-5"
                  />
                </View>
                <View class="h-px bg-border/70" />
                <View class="flex-row items-center justify-between gap-3 px-4 py-3">
                  <View style={{ flex: 1, gap: 2 }}>
                    <Text class="text-[12px] font-medium text-foreground">隐藏程序坞图标</Text>
                    <Text class="text-[10px] leading-4 text-muted-foreground">
                      只在菜单栏运行，不占 Dock
                    </Text>
                  </View>
                  <Switch
                    checked={settings().hideDockIcon}
                    onChange={({ checked }) => updateSettings({ hideDockIcon: checked })}
                    accessibilityLabel="隐藏程序坞图标"
                    trackClass="h-6 w-11 rounded-full bg-muted p-0.5 transition-colors checked:bg-primary"
                    thumbClass="size-5 rounded-full bg-background shadow-sm transition-transform checked:translate-x-5"
                  />
                </View>
              </Card>

              <Card style={{ padding: 0 }}>
                <View class="gap-1 px-4 py-3">
                  <View class="flex-row items-center gap-2">
                    <Text class="text-[12px] font-medium text-foreground">强制休息</Text>
                    <If when={forceNext()}>
                      <Text class="rounded-full bg-muted px-2 py-0.5 text-[9px] font-medium text-foreground">
                        下一次
                      </Text>
                    </If>
                    <If when={postpones() > 0}>
                      <Text class="text-[10px] text-muted-foreground">已延后 {postpones()} 次</Text>
                    </If>
                  </View>
                  <Text class="text-[10px] leading-4 text-muted-foreground">
                    锁定休息时每次都强制燃尽；关闭锁定后，连续延后 3 次或跳过 1 次，下一次也无法提前结束。
                  </Text>
                </View>
              </Card>
            </View>
          </If>
        </Scroll>

        <View class="h-px bg-border" />
        <View class="h-9 flex-row items-center justify-between px-3">
          <View class="flex-row items-center gap-1.5">
            <Text class="text-[10px] text-muted-foreground">
              今日 {stats().focusMinutes} 分钟
            </Text>
            <Text class="text-[10px] text-faint">·</Text>
            <Text class="text-[10px] text-faint">香篆</Text>
          </View>
          <Button
            onPress={() => Application.quit()}
            class="rounded-[7px] px-2 py-1 transition duration-150 hover:bg-accent active:scale-95"
          >
            <Text class="text-[11px] text-muted-foreground">退出</Text>
          </Button>
        </View>
      </View>
    </View>
  );
}

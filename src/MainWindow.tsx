import {
  cleanup,
  createAnimatedValue,
  createMotion,
  Easing,
  effect,
  keymap,
  reduceMotion,
  Scroll,
  Text,
  View,
} from "nsh";
import { Switch } from "nsh/base";
import { currentWindow, titleBar } from "nsh/desktop";
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
  settings,
  setWorkMinutes,
  showApp,
  skipRest,
  startFocus,
  startRest,
  stats,
  stopCycle,
  updateSettings,
} from "./state";
import { setLoginItem } from "./login-item";
import { Action, Card, Dot, KeyHint, Ring, SectionTitle, Segmented } from "./ui";

const WORK_OPTIONS = [25, 45, 60] as const;
const REST_OPTIONS = [60, 90, 120] as const;
const POSTPONE_OPTIONS = [5, 10, 15] as const;
const SKIP_OPTIONS = [5, 10, 15, 20] as const;

export function MainWindow() {
  const win = currentWindow();
  const ring = createAnimatedValue(1);
  const head = createMotion({ enter: "fadeInUp", duration: 520 });
  const body = createMotion({ enter: "fadeInUp", duration: 560, delay: 70 });

  effect(() => {
    win.set({ closeOnPress: false, title: "香篆" });
    const stop = win.on("closePress", () => win.hide());
    cleanup(stop);
  });

  // A rest owns the screen. Hide the main window even if it mounted after
  // `startRest` ran, and never show it again when the rest ends.
  effect(() => {
    if (phase() === "rest") win.hide();
  });

  let shownPart = 1;
  effect(() => {
    const part = remainingPart();
    if (part === shownPart) return;
    shownPart = part;
    if (reduceMotion()) ring.set(part);
    else ring.timing(part, { duration: 480, easing: Easing.linear });
  });

  const isRest = () => phase() === "rest";
  const isIdle = () => phase() === "idle";
  const isFocus = () => phase() === "focus";
  const isPaused = () => isFocus() && paused();

  return (
    <View style={{ flex: 1, backgroundColor: "transparent" }}>
      <View
        desktop={{ windowDrag: true }}
        style={{
          height: titleBar().height || 52,
          paddingLeft: titleBar().left + 14,
          paddingRight: 16,
          flexDirection: "row",
          alignItems: "center",
          gap: 10,
        }}
      >
        <View class="size-6 items-center justify-center rounded-[7px] border border-primary/20 bg-primary/10">
          <Text
            style={{ fontFamily: "Songti SC", fontSize: 13, lineHeight: 16 }}
            class="text-primary"
          >
            香
          </Text>
        </View>
        <Text class="text-[13px] font-semibold tracking-[0.08em] text-foreground">香篆</Text>
        <View class="flex-1" />
        <Text class="text-[9px] uppercase tracking-[0.22em] text-faint">Xiangzhuan</Text>
      </View>

      <Scroll style={{ flex: 1 }} scrollbar={false} contentStyle={{ padding: 20, paddingTop: 4, paddingBottom: 22, gap: 16 }}>
        <View style={{ ...head.style, gap: 16 }}>
          <Card class="items-center" style={{ paddingTop: 20, paddingBottom: 20 }}>
            <View class="items-center gap-4">
              <View class="flex-row items-center gap-2">
                <Dot tone={isRest() ? "rest" : isIdle() ? "idle" : "work"} />
                <Text class="text-[10px] uppercase tracking-[0.24em] text-muted-foreground">
                  {phaseLabel()}
                </Text>
              </View>

              <Ring progress={ring} size={236} tone="main">
                <Text
                  class="text-[54px] font-light leading-none tracking-[-0.045em] text-countdown tabular-nums"
                >
                  {formatClock(remainingMs())}
                </Text>
                <Text class="mt-2 text-[11px] tracking-[0.04em] text-muted-foreground">
                  {isIdle() ? `一炷香 · ${settings().workMinutes} 分钟` : isPaused() ? "已暂停 · 点继续" : phaseLine()}
                </Text>
              </Ring>

              <If when={isRest()}>
                <View class="items-center gap-3">
                  <Text class="text-center text-[12px] leading-5 text-muted-foreground">
                    {phaseLine()}
                  </Text>
                  <View class="flex-row gap-2">
                    <If when={canPostponeNow()}>
                      <Action label={`延后 ${settings().postponeMinutes} 分钟`} onPress={postponeRest} />
                    </If>
                    <If when={canSkipNow()}>
                      <Action label="提前结束" variant="ghost" onPress={skipRest} />
                    </If>
                    <If when={!canPostponeNow() && !canSkipNow()}>
                      <Text class="text-[11px] tracking-[0.12em] text-warning">
                        {restForced()
                          ? "强制休息中"
                          : restInputLocked()
                            ? "锁定休息中"
                            : settings().allowSkip && stats().skips >= settings().maxDailySkips
                              ? "今日跳过已达上限"
                              : "静候香尽"}
                      </Text>
                    </If>
                  </View>
                  <Action label="回到休息屏" variant="ghost" onPress={showApp} />
                </View>
              </If>

              <If when={!isRest()}>
                <View class="flex-row items-center gap-2">
                  <If when={isIdle()}>
                    <Action label="开始专注" variant="primary" onPress={startFocus} testID="start-focus" />
                  </If>
                  <If when={isFocus()}>
                    <Action
                      label={isPaused() ? "继续专注" : "暂停专注"}
                      variant="primary"
                      onPress={isPaused() ? resumeFocus : pauseFocus}
                      testID={isPaused() ? "resume-focus" : "pause-focus"}
                    />
                  </If>
                  <If when={!isIdle()}>
                    <Action
                      label="立即休息"
                      variant={isFocus() ? "ghost" : "primary"}
                      onPress={() => startRest(false)}
                      testID="rest-now"
                    />
                  </If>
                  <If when={isIdle()}>
                    <Action label="直接休息" variant="ghost" onPress={() => startRest(false)} />
                  </If>
                  <If when={!isIdle()}>
                    <Action label="结束本轮" variant="ghost" onPress={stopCycle} />
                  </If>
                </View>
                <View class="flex-row items-center gap-2">
                  <KeyHint keys={keymap.label("mod+shift+p")} />
                  <Text class="text-[10px] text-faint">暂停 / 继续</Text>
                  <View class="mx-1 h-3 w-px bg-border" />
                  <KeyHint keys="Esc" />
                  <Text class="text-[10px] text-faint">休息时提前结束</Text>
                </View>
              </If>
            </View>
          </Card>

          <View class="flex-row gap-3">
            <View class="flex-1 rounded-(--radius) border border-border/60 bg-card/55 px-4 py-3">
              <Text class="text-[10px] uppercase tracking-[0.18em] text-muted-foreground">今日专注</Text>
              <Text class="mt-1 text-[20px] font-medium tabular-nums text-foreground">
                {stats().focusMinutes}
                <Text class="ml-1 text-[11px] font-normal text-muted-foreground">分钟</Text>
              </Text>
            </View>
            <View class="flex-1 rounded-(--radius) border border-border/60 bg-card/55 px-4 py-3">
              <Text class="text-[10px] uppercase tracking-[0.18em] text-muted-foreground">完成休息</Text>
              <Text class="mt-1 text-[20px] font-medium tabular-nums text-foreground">
                {stats().rests}
                <Text class="ml-1 text-[11px] font-normal text-muted-foreground">次</Text>
              </Text>
            </View>
          </View>
        </View>

        <View style={{ ...body.style, gap: 16 }}>
          <View class="gap-2">
            <SectionTitle hint={!isIdle() ? "本轮进行中" : undefined}>节奏</SectionTitle>
            <Card style={{ padding: 16, gap: 14 }}>
              <View class="gap-2">
                <View class="flex-row items-center justify-between">
                  <Text class="text-[12px] font-medium text-foreground">专注</Text>
                  <Text class="text-[10px] text-faint">一炷香</Text>
                </View>
                <Segmented
                  label="专注时长"
                  value={settings().workMinutes}
                  options={WORK_OPTIONS.map((m) => ({ value: m, label: `${m} 分` }))}
                  onChange={(workMinutes) => setWorkMinutes(workMinutes)}
                  disabled={isRest()}
                />
              </View>
              <View class="h-px bg-border/60" />
              <View class="gap-2">
                <View class="flex-row items-center justify-between">
                  <Text class="text-[12px] font-medium text-foreground">休息</Text>
                  <Text class="text-[10px] text-faint">{settings().restSeconds} 秒</Text>
                </View>
                <Segmented
                  label="休息时长"
                  value={settings().restSeconds}
                  options={REST_OPTIONS.map((s) => ({ value: s, label: `${s} 秒` }))}
                  onChange={(restSeconds) => updateSettings({ restSeconds })}
                />
              </View>
              <View class="h-px bg-border/60" />
              <View class="gap-2">
                <View class="flex-row items-center justify-between">
                  <Text class="text-[12px] font-medium text-foreground">延后</Text>
                  <Text class="text-[10px] text-faint">急事时先收起休息屏</Text>
                </View>
                <Segmented
                  label="延后时长"
                  value={settings().postponeMinutes}
                  options={POSTPONE_OPTIONS.map((m) => ({ value: m, label: `${m} 分` }))}
                  onChange={(postponeMinutes) => updateSettings({ postponeMinutes })}
                />
              </View>
            </Card>
          </View>

          <View class="gap-2">
            <SectionTitle>规则</SectionTitle>
            <Card style={{ padding: 16, gap: 14 }}>
              <View class="flex-row items-center justify-between gap-4">
                <View style={{ flex: 1, gap: 2 }}>
                  <Text class="text-[12px] font-medium text-foreground">休息时锁定输入</Text>
                  <Text class="text-[10px] leading-4 text-muted-foreground">
                    全屏休息时拦截键盘鼠标与切屏，只能等香燃尽
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
              <View class="h-px bg-border/60" />
              <View class="flex-row items-center justify-between gap-4">
                <View style={{ flex: 1, gap: 2 }}>
                  <Text class="text-[12px] font-medium text-foreground">允许跳过休息</Text>
                  <Text class="text-[10px] leading-4 text-muted-foreground">
                    {settings().lockRest ? "锁定休息时不可用" : "关掉后，只能等香燃尽或延后"}
                  </Text>
                </View>
                <Switch
                  checked={settings().allowSkip}
                  disabled={settings().lockRest}
                  onChange={({ checked }) => updateSettings({ allowSkip: checked })}
                  accessibilityLabel="允许跳过休息"
                  trackClass="h-6 w-11 rounded-full bg-muted p-0.5 transition-colors checked:bg-primary"
                  thumbClass="size-5 rounded-full bg-background shadow-sm transition-transform checked:translate-x-5"
                />
              </View>
              <View class="h-px bg-border/60" />
              <View style={{ gap: 8 }}>
                <View class="flex-row items-center justify-between">
                  <Text class="text-[12px] font-medium text-foreground">每日跳过上限</Text>
                  <Text class="text-[10px] text-faint">
                    今日已跳 {stats().skips} / {settings().maxDailySkips} 次
                  </Text>
                </View>
                <Segmented
                  label="每日跳过上限"
                  value={settings().maxDailySkips}
                  options={SKIP_OPTIONS.map((n) => ({ value: n, label: `${n} 次` }))}
                  onChange={(maxDailySkips) => updateSettings({ maxDailySkips })}
                />
              </View>
              <View class="h-px bg-border/60" />
              <View class="flex-row items-center justify-between gap-4">
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
              <View class="h-px bg-border/60" />
              <View class="flex-row items-center justify-between gap-4">
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
              <View class="h-px bg-border/60" />
              <View style={{ gap: 4 }}>
                <View class="flex-row items-center gap-2">
                  <Text class="text-[12px] font-medium text-foreground">强制休息</Text>
                  <If when={forceNext()}>
                    <Text class="rounded-full bg-warning/15 px-2 py-0.5 text-[9px] font-medium text-warning">
                      下一次
                    </Text>
                  </If>
                </View>
                <Text class="text-[10px] leading-4 text-muted-foreground">
                  锁定休息时每次都强制燃尽；关闭锁定后，连续延后 3 次或跳过 1 次，下一次也无法提前结束
                </Text>
                <If when={postpones() > 0}>
                  <Text class="text-[10px] text-muted-foreground">
                    本轮已延后 {postpones()} 次
                  </Text>
                </If>
              </View>
            </Card>
          </View>

          <View class="flex-row items-center justify-center gap-2 py-1">
            <Text class="text-[9px] uppercase tracking-[0.18em] text-faint">
              One stick at a time
            </Text>
            <Text class="text-[9px] text-faint">·</Text>
            <Text class="text-[9px] text-faint">香篆 {stats().skips > 0 ? `· 今日跳过 ${stats().skips} 次` : ""}</Text>
          </View>
        </View>
      </Scroll>
    </View>
  );
}

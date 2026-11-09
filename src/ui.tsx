import { Canvas, token, type AnimatedValue, type JSX, type StyleProp, type ViewStyle } from "nsh";
import { Button } from "nsh/base";
import { Text, View } from "nsh";

export type ActionVariant = "primary" | "outline" | "ghost" | "danger";

const actionClass: Record<ActionVariant, string> = {
  primary:
    "bg-primary text-primary-foreground shadow-sm hover:bg-primary/90 active:bg-primary/85",
  outline:
    "border border-border bg-card/70 text-foreground hover:bg-accent/60 active:bg-accent",
  ghost: "text-muted-foreground hover:bg-accent/55 active:bg-accent/80",
  danger:
    "bg-destructive text-white shadow-sm hover:bg-destructive/90 active:bg-destructive/85",
};

export function Action(props: {
  label?: string;
  onPress?: () => void;
  variant?: ActionVariant;
  disabled?: boolean;
  children?: JSX.Element;
  class?: string;
  style?: StyleProp<ViewStyle>;
  accessibilityLabel?: string;
  testID?: string;
}) {
  return (
    <Button
      onPress={props.onPress}
      disabled={props.disabled}
      accessibilityLabel={props.accessibilityLabel ?? props.label}
      testID={props.testID}
      class={`flex-row items-center justify-center gap-2 rounded-(--radius) px-4 py-2.5 transition duration-150 active:scale-95 disabled:opacity-40 ${actionClass[props.variant ?? "outline"]} ${props.class ?? ""}`}
      style={props.style}
    >
      {props.children ?? (
        <Text class="text-[13px] font-medium tracking-[0.01em]">{props.label}</Text>
      )}
    </Button>
  );
}

export interface Option<T> {
  value: T;
  label: string;
}

export function Segmented<T extends string | number>(props: {
  value: T;
  options: readonly Option<T>[];
  onChange: (value: T) => void;
  disabled?: boolean;
  label?: string;
}) {
  return (
    <View
      accessibilityLabel={props.label}
      class="flex-row gap-1 rounded-(--radius) bg-muted/75 p-1"
    >
      {props.options.map((option) => {
        const selected = option.value === props.value;
        return (
          <Button
            selected={selected}
            disabled={props.disabled}
            accessibilityLabel={`${props.label ?? ""} ${option.label}`.trim()}
            onPress={() => props.onChange(option.value)}
            class={`flex-1 items-center justify-center rounded-[10px] px-3 py-1.5 transition duration-150 active:scale-95 ${
              selected
                ? "bg-card text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Text class="text-[12px] font-medium tabular-nums">{option.label}</Text>
          </Button>
        );
      })}
    </View>
  );
}

export function Card(props: {
  children?: JSX.Element;
  class?: string;
  style?: StyleProp<ViewStyle>;
}) {
  return (
    <View
      class={`rounded-(--radius-card) border border-border/70 bg-card/85 p-5 shadow-sm ${props.class ?? ""}`}
      style={props.style}
    >
      {props.children}
    </View>
  );
}

export function SectionTitle(props: { children?: JSX.Element | string; hint?: string }) {
  return (
    <View class="flex-row items-baseline justify-between px-1">
      <Text class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
        {props.children}
      </Text>
      {props.hint ? <Text class="text-[11px] text-faint">{props.hint}</Text> : null}
    </View>
  );
}

export function KeyHint(props: { keys: string }) {
  return (
    <View class="flex-row items-center gap-1">
      {props.keys.split("+").map((key) => (
        <View class="rounded-[5px] border border-border/80 bg-card/80 px-1.5 py-0.5">
          <Text class="text-[10px] font-medium text-muted-foreground">{key}</Text>
        </View>
      ))}
    </View>
  );
}

export function Ring(props: {
  progress: AnimatedValue;
  size: number;
  children?: JSX.Element;
  tone?: "main" | "rest";
}) {
  return (
    <View
      style={{ width: props.size, height: props.size, position: "relative", alignItems: "center", justifyContent: "center" }}
    >
      <Canvas
        style={{ position: "absolute", left: 0, top: 0, width: props.size, height: props.size, pointerEvents: "none" }}
        draw={(cx, { width, height }) => {
          const centerX = width / 2;
          const centerY = height / 2;
          const radius = Math.min(width, height) / 2 - Math.max(18, width * 0.045);
          const tickCount = props.tone === "rest" ? 72 : 48;
          const rest = props.tone === "rest";
          const track = rest ? "rgba(244,231,213,0.16)" : token("ring-track");
          const progress = rest ? "#e08a5a" : token("ring-progress");
          const glow = rest ? "rgba(224,138,90,0.55)" : token("ember-glow");

          cx.clearRect(0, 0, width, height);

          cx.lineWidth = 1;
          cx.globalAlpha = 0.85;
          cx.strokeStyle = track;
          for (let i = 0; i < tickCount; i++) {
            const angle = (i / tickCount) * Math.PI * 2 - Math.PI / 2;
            const inner = radius + 6;
            const outer = radius + (i % 6 === 0 ? 14 : 10);
            cx.beginPath();
            cx.moveTo(centerX + Math.cos(angle) * inner, centerY + Math.sin(angle) * inner);
            cx.lineTo(centerX + Math.cos(angle) * outer, centerY + Math.sin(angle) * outer);
            cx.stroke();
          }
          cx.globalAlpha = 1;

          cx.lineWidth = 1.5;
          cx.strokeStyle = track;
          cx.beginPath();
          cx.arc(centerX, centerY, radius, 0, Math.PI * 2);
          cx.stroke();

          cx.lineWidth = props.tone === "rest" ? 6 : 4;
          cx.lineCap = "round";
          cx.strokeStyle = progress;
          cx.shadowColor = glow;
          cx.shadowBlur = props.tone === "rest" ? 24 : 16;
          const end = props.progress.mul(Math.PI * 2).sub(Math.PI / 2);
          cx.beginPath();
          cx.arc(centerX, centerY, radius, -Math.PI / 2, end);
          cx.stroke();
          cx.shadowBlur = 0;

          cx.lineWidth = 1;
          cx.globalAlpha = 0.5;
          cx.strokeStyle = track;
          cx.beginPath();
          cx.arc(centerX, centerY, radius * 0.78, 0, Math.PI * 2);
          cx.stroke();
          cx.globalAlpha = 1;
        }}
      />
      <View style={{ alignItems: "center", justifyContent: "center" }}>
        {props.children}
      </View>
    </View>
  );
}

export function Dot(props: { tone?: "idle" | "work" | "rest" }) {
  const color =
    props.tone === "work"
      ? "bg-primary"
      : props.tone === "rest"
        ? "bg-ember"
        : "bg-muted-foreground";
  return <View class={`size-1.5 rounded-full ${color}`} />;
}

# 香篆 (Xiangzhuan)

[![CI](https://github.com/iwangjie/xiangzhuan/actions/workflows/ci.yml/badge.svg)](https://github.com/iwangjie/xiangzhuan/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/iwangjie/xiangzhuan)](https://github.com/iwangjie/xiangzhuan/releases)

> 一篆香消，万事且抛。

驻在 macOS 菜单栏的休息提醒：工作到点，全屏燃香；一炷香尽，回到案前。

![休息屏](docs/0.2.0/rest-no-skip.png)

## 是什么

香篆常驻菜单栏，按设定的工作时长倒计时。时间一到，屏幕淡入一方篆印与一柱燃香：香头从起点匀速烧向终点，烧尽自动回到下一轮工作。休息屏是无边框覆盖层，盖住菜单栏与 Dock，三指左右滑动也切不走；想提前结束，按 `Esc` 拂灰起行（也可以在设置里关上这条路）。

原生界面（[MyGo](https://github.com/egoist/mygo) native UI，无 webview），没有网络请求、没有账号、不收集任何数据。

## 功能

- 工作 30–50 分钟、休息 60–120 秒，随时调整；改完即存，下一轮生效
- 「允许跳过休息」可关：关掉后休息屏不出现跳过按钮，`Esc` 也失效，真正独占禅定
- 菜单栏篆印图标 + 剩余分钟读数（最后一分钟显示秒，休息时显示「休」）
- 全屏覆盖层休息屏：淡入淡出、零黑帧、滑动切不走
- 全中文界面；退出干净
- macOS 12+ · Apple Silicon（arm64）

## 安装

在 [Releases](https://github.com/iwangjie/xiangzhuan/releases/latest) 下载 `香篆 x.y.z.dmg`，打开后把「香篆」拖进「应用程序」。

应用未签名，首次打开如被 Gatekeeper 拦下，右键点选「打开」；或在终端执行：

```sh
xattr -d com.apple.quarantine /Applications/香篆.app
```

## 使用

- 点菜单栏篆印图标：打开香篆 / 开始工作（按设置的分钟数）/ 即刻休息（按设置的秒数）/ 跳过休息 / 退出香篆
- 主窗口里调整工作时长、休息时长与是否允许跳过，改动即时保存
- 休息中按 `Esc` 或点「拂灰起行」提前结束（须允许跳过）

![设置界面](docs/0.2.0/settings-window.png)

## 从源码构建

需要 macOS 与 Go 1.27.1+（无需 Xcode）：

```sh
git clone https://github.com/iwangjie/xiangzhuan.git
cd xiangzhuan
CGO_ENABLED=0 go tool mygo build    # → build/darwin-arm64/香篆.app 与 DMG
```

测试与静态检查：

```sh
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...
```

> 加 `CGO_ENABLED=0` 是为了绕开部分机器上 CLT/Xcode 链接器不匹配的问题，CI 里同样如此。

## 许可

[MIT](LICENSE) © 2026 wangjie

---

# Xiangzhuan

> One stick of incense burns down; let the world wait.

A break reminder that lives in your macOS menu bar. When work time is up, a stick of incense burns across the screen; when it burns out, you are back to work.

## What it is

Xiangzhuan (香篆, "incense seal") sits in the menu bar and counts down your work session. When time is up, the screen fades into a seal and a burning stick of incense — the ember travels at a steady pace along the path; when it reaches the end, the next work session begins. The rest screen is a borderless overlay that covers the menu bar and Dock and cannot be swiped away with a three-finger gesture. To end a rest early, press `Esc` to brush the ash aside (this can be disallowed in settings).

Native UI ([MyGo](https://github.com/egoist/mygo) native, no webview): no network requests, no account, no telemetry.

## Features

- Work 30–50 min, rest 60–120 s; changes save immediately and apply from the next round
- "Allow skipping" can be turned off: no skip button, `Esc` disabled — a true full-screen retreat
- Menu bar seal icon with minutes left (seconds in the last minute; 休 during rest)
- Full-screen overlay rest screen: fades in and out, no black frames, cannot be swiped away
- Chinese UI throughout; quits cleanly
- macOS 12+ · Apple Silicon (arm64)

## Install

Download `香篆 x.y.z.dmg` from [Releases](https://github.com/iwangjie/xiangzhuan/releases/latest) and drag 香篆 into Applications.

The app is unsigned; if Gatekeeper blocks the first launch, right-click and choose Open, or run:

```sh
xattr -d com.apple.quarantine /Applications/香篆.app
```

## Usage

- Click the menu bar icon: Open / Start work (your minutes) / Rest now (your seconds) / Skip rest / Quit
- Adjust work and rest length and the skip permission in the main window; saved instantly
- Press `Esc` or click the button to end a rest early (when skipping is allowed)

## Build from source

Requires macOS and Go 1.27.1+ (Xcode not needed):

```sh
git clone https://github.com/iwangjie/xiangzhuan.git
cd xiangzhuan
CGO_ENABLED=0 go tool mygo build    # → build/darwin-arm64/香篆.app and a DMG
```

Tests and vet:

```sh
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...
```

> `CGO_ENABLED=0` avoids a CLT/Xcode linker mismatch seen on some machines; CI uses it too.

## License

[MIT](LICENSE) © 2026 wangjie

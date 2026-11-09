# 香篆 (Xiangzhuan)

> 一篆香消，万事且抛。

驻在 macOS 菜单栏的休息提醒。工作到点，全屏燃香；一炷香尽，回到案前。

这一版用 [Native Shell](https://nsh.dev)（nsh）从头重写：AppKit 原生视图、TSX 界面、nsh 的窗口 / 托盘 / 动画 / 菜单 API。全屏休息屏是一块内联的 three.js 场景（WebView），没有网络请求，没有账号，不收集数据。

## 新设计

- **菜单栏是家**：篆印常驻菜单栏，左键唤出香篆，右键直接开始专注、立即休息、延后或退出。
- **一炷香的主界面**：暖纸与墨色，状态、快捷键和今日专注都在一屏里。
- **全屏休息**：每块显示器各开一块无边框休息屏，盖住菜单栏与 Dock，跟随所有 Space；three.js 渲染一间室内茶室/香室，一炷香在案上缓慢燃烧，烟从香头升起。开场先停留在整间茶室，约 3 秒后缓慢推近到香头特写；可切换「茶席 / 香篆 / 书画」三个视角。
- **每次不一样的字画**：墙上挂轴从一组通透格言里随机抽一条，毛笔竖排，挂位、倾斜、题款和朱印每次休息都不同。
- **可延后，也可强制**：延后 5/10/15 分钟后重新完整休息；连续延后 3 次或跳过 1 次，下一次休息强制燃尽，不再提供提前结束。
- **nsh 的交互动效**：窗口、按钮、分段选择、开关、转场全部使用 nsh 原生状态样式、`createMotion` 与 `createAnimatedValue`；休息屏的计数、跳过与延后经 WebView 消息桥回到 nsh。
- **安静地住在菜单栏**：关闭主窗口只是收起；登录时启动默认开启（LaunchAgent），不占 Dock。

## 功能

- 专注 25/45/60 分钟，休息 60/90/120 秒，延后 5/10/15 分钟
- 工作到点自动进入全屏休息；休息结束自动开始下一轮专注
- 「立即休息」随时开始一次完整休息
- 「允许跳过休息」可关：关掉后只能等香燃尽或延后
- 每块显示器独立的无边框休息窗口，同进同出
- 菜单栏工具提示显示剩余时间；菜单里显示状态与可执行动作
- `Esc` 提前结束（允许跳过且非强制休息时）
- 菜单快捷：`⇧⌘S` 开始专注，`⇧⌘R` 立即休息
- 设置与今日统计写入 `~/Library/Application Support/com.iwangjie.xiangzhuan/storage.json`
- macOS 14+（nsh 的要求）

## 从源码运行

需要 macOS、Rust 工具链与 nsh：

```sh
curl -fsSL https://nsh.dev/install.sh | sh
cd xiangzhuan
nsh run App.tsx
```

`nsh run` 会打开开发宿主并热更新。

## 打包与安装

```sh
nsh package App.tsx --target macos
open .nsh/out/香篆.app
```

装进 `/Applications`：

```sh
rm -rf /Applications/香篆.app
cp -R .nsh/out/香篆.app /Applications/
open -a 香篆
```

制作可分发的 DMG（内含 App 与 Applications 快捷方式，拖入即装）：

```sh
rm -rf /tmp/xiangzhuan-dmg
mkdir -p /tmp/xiangzhuan-dmg
cp -R .nsh/out/香篆.app /tmp/xiangzhuan-dmg/
ln -s /Applications /tmp/xiangzhuan-dmg/Applications
hdiutil create -volname "香篆" -srcfolder /tmp/xiangzhuan-dmg -ov -format UDZO .nsh/out/香篆-macos.dmg
```

未签名应用首次打开如被 Gatekeeper 拦下，右键点选「打开」，或：

```sh
xattr -d com.apple.quarantine /Applications/香篆.app
```

## 项目结构

```text
nsh.json          应用名、bundle id、窗口、macOS Info.plist
theme.json        暖纸 / 墨 / 朱砂的主题 token（亮暗两套）
App.tsx           入口：主窗口 + 休息窗口 + 菜单栏 Shell
src/state.ts      计时状态机、设置、统计、持久化
src/Shell.tsx     菜单栏图标、菜单、应用菜单、快捷键、计时器
src/MainWindow.tsx  主界面
src/RestWindows.tsx 多显示器全屏休息屏（WebView）
src/shrine-html.ts 内联 three.js 场景（由 scripts/embed-shrine.py 生成）
scripts/embed-shrine.py 把 HTML 与 three.js 打包成一个 TS 字符串
test/rest-shrine.html 休息屏场景源文件（three.js 单文件，打包时内联 three）
src/ui.tsx        按钮、分段选择、Ring、卡片等设计系统
src/login-item.ts 登录时启动（LaunchAgent）
assets/           图标与菜单栏模板
```

## 许可

[MIT](LICENSE) © 2026 wangjie

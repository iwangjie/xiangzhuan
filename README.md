# 香篆

驻在 macOS 菜单栏的专注与休息提醒。工作到点，全屏燃香；一炷香尽，回到案前。

用 [nsh](https://nsh.dev) 写成：AppKit 原生视图、TSX 界面，休息屏是一块内联的 three.js 场景（WebView）。没有网络请求，没有账号，不收集数据。

- bundle id：`com.iwangjie.xiangzhuan`
- 系统：macOS 14+

## 现在是什么样

**菜单栏是家。** 启动即以 Accessory 方式驻留菜单栏，不占 Dock（可在设置里切回），并立刻开始一轮专注。关闭主窗口只是收起。

**一炷香。** 主界面显示这一轮还剩多久、今天专注了多久、香烧到哪里。专注 25/45/60 分钟可选，中途改时长按剩余比例重算，不打断当前的节奏。

**随时暂停。** 主界面按钮、菜单栏菜单、`⇧⌘P` 都能暂停与继续；暂停期间不计时，继续时把剩余时间原样给回。

**到点休息。** 每块显示器各开一块无边框全屏休息屏，盖住菜单栏与 Dock，跟随所有 Space，同进同出。

**休息屏里的一间静室。** three.js 画一间室内茶席：茶、香、书画，元素不多。开场停整屋 3 秒，再用 3 秒推近到香头；可切「茶席 / 香篆 / 书画」三个视角，拖动可环视。

**香是真的在烧。** 线香随剩余时间同步变短，火头带炭化与白灰帽，青烟成缕上升；燃烧的灰隔一阵整片落下，匀速、翻转，落进香炉。

**燃尽收尾。** 香烧完，镜头移到茶席，紫砂壶提起、倾斜、出汤，白瓷杯里液面上升、起涟漪、冒热气；随后镜头俯身停在一杯茶，整场光缓缓退去，黑场结束，自动开始下一轮专注。

**每次不一样的字画。** 墙上挂轴从一组通透格言里随机抽一条，毛笔竖排，挂位、题款、朱印每次都不同。

**延后与跳过都有边界。** 延后 5/10/15 分钟后重新开始一轮完整专注；连续延后 3 次或跳过 1 次，下一次休息强制燃尽，不再提供提前结束。每日跳过次数默认上限 10 次（可设为 5/10/15/20），用完后当天只能休息或延后。

**不卡。** 启动 1.4 秒后在后台预热 three.js 页面，进入休息只是显示窗口；四核及以下自动降采样（像素比 1.0、关抗锯齿、贴图 512、粒子减半）。

## 快捷键

| 快捷键 | 作用 |
| --- | --- |
| `⇧⌘S` | 开始专注（待燃时） |
| `⇧⌘P` | 暂停 / 继续专注 |
| `⇧⌘R` | 立即休息 |
| `Esc` | 提前结束休息（非强制且当日跳过次数未用尽时） |

## 运行

需要 macOS 与 nsh：

```sh
curl -fsSL https://nsh.dev/install.sh | sh
nsh run App.tsx
```

## 打包

```sh
nsh package App.tsx --target macos
sh scripts/install-dock-hide.sh .nsh/out/香篆.app
```

第二行把 `native/dock_hide.m` 编译成 dylib 注入 App：它每 0.8 秒读一次设置里的 `hideDockIcon`，实时切换「只在菜单栏」与「显示在 Dock」。**每次重新打包都要再跑一次**，否则安装的是一个没有签名过的、Dock 图标永远显示的版本。

装进 `/Applications`：

```sh
pkill -f '/Applications/香篆.app/Contents/MacOS'
rm -rf /Applications/香篆.app
cp -R .nsh/out/香篆.app /Applications/
xattr -dr com.apple.quarantine /Applications/香篆.app
open -g /Applications/香篆.app
```

做可分发的 DMG（App 与 Applications 快捷方式在同一个窗口里，拖进去即装）：

```sh
rm -rf /tmp/xiangzhuan-dmg
mkdir -p /tmp/xiangzhuan-dmg
cp -R .nsh/out/香篆.app /tmp/xiangzhuan-dmg/
ln -s /Applications /tmp/xiangzhuan-dmg/Applications
hdiutil create -volname "香篆" -srcfolder /tmp/xiangzhuan-dmg -ov -format UDZO .nsh/out/香篆-macos.dmg
```

未签名应用首次打开如被 Gatekeeper 拦下：右键「打开」，或 `xattr -d com.apple.quarantine /Applications/香篆.app`。

## 结构

```text
nsh.json                 应用名、bundle id、窗口、Info.plist、能力声明
theme.json               暖纸 / 墨 / 朱砂主题 token
App.tsx                  入口：主窗口 + 休息窗口 + 菜单栏 Shell
src/state.ts             计时状态机、设置、统计、持久化
src/Shell.tsx            托盘图标与菜单、应用菜单、快捷键、250ms 心跳
src/MainWindow.tsx       主界面
src/RestWindows.tsx      多显示器全屏休息屏（WebView 宿主）
src/ui.tsx               按钮、分段选择、开关、卡片等设计系统
src/login-item.ts        登录时启动（LaunchAgent）
src/shrine-html.ts       休息屏页面（生成物，勿手改）
test/rest-shrine.html    休息屏场景源码（单文件 three.js）
scripts/embed-shrine.py  把场景与 three.js 内联成 src/shrine-html.ts
scripts/install-dock-hide.sh  编译并注入 Dock 隐藏 dylib
native/dock_hide.m       Dock 显示策略的 swizzle 实现
assets/                  图标、菜单栏模板、three.min.js
```

数据落在 `~/Library/Application Support/com.iwangjie.xiangzhuan/storage.json`。

## 许可

[MIT](LICENSE) © 2026 wangjie

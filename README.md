# 香篆

驻在 macOS 菜单栏和 Windows 系统托盘的专注与休息提醒。工作到点，全屏燃香；一炷香尽，回到案前。

用 [nsh](https://nsh.dev) 写成：两端共用 TSX 界面和 WebView 中的离线 three.js 休息场景。应用通过 GitHub 检查更新，无需账号，也不收集数据。

- bundle id：`com.iwangjie.xiangzhuan`
- 系统：macOS 14+，Windows 10 1809+ 或 Windows 11（Windows 需要 x64 和 WebView2 Runtime）

## 现在是什么样

**启动即专注。** macOS 驻留菜单栏，默认隐藏 Dock 图标。Windows 驻留系统托盘。两端都在启动时开始专注，关闭主窗口时收起界面。

**一炷香。** 主界面显示这一轮还剩多久、今天专注了多久、香烧到哪里。专注 25/45/60 分钟可选，中途改时长按剩余比例重算，不打断当前的节奏。

**随时暂停。** 主界面按钮、菜单栏菜单、`⇧⌘P` 都能暂停与继续；暂停期间不计时，继续时把剩余时间原样给回。

**空闲自动暂停。** 专注期间，键鼠连续 5 分钟没有操作就暂停计时。回来操作时继续剩余时间；若整段空闲已达到设置的专注时长，就从完整的一轮重新开始。

**到点休息。** 每块显示器各开一块无边框全屏休息屏。macOS 的休息屏覆盖菜单栏和 Dock，跟随所有 Space。默认开启「休息时锁定输入」，休息期间拦截键鼠与切屏，香燃尽后回到休息前的 App。

Windows 的「锁定休息」会将休息屏置顶，并关闭跳过、延后和场景交互。系统切换键由 Windows 处理。两端都可以在设置里解除锁定。

**休息结束不弹主界面。** 香燃尽、跳过或延后后，主窗口保持收起。需要时可从菜单栏或系统托盘打开香篆。

**休息屏里的一间静室。** three.js 画一间室内茶席：茶、香、书画。开场停整屋 3 秒，再用 3 秒推近到香头；可切「茶席 / 香篆 / 书画」三个视角，拖动可环视。锁定休息时这些交互控件会收起。大肥鱼有时会来茶室，跑进来或先在画面边缘探头，随机喝茶、看画或在角落张望，做完再沿路离开。角色素材来源见 [大肥鱼素材署名](assets/fish/NOTICE.md)。

**香是真的在烧。** 线香随剩余时间同步变短，火头带炭化与白灰帽，青烟成缕上升；燃烧的灰隔一阵整片落下，匀速、翻转，落进香炉。

**燃尽收尾。** 香烧完，镜头移到茶席，紫砂壶提起、倾斜、出汤，白瓷杯里液面上升、起涟漪、冒热气；随后镜头俯身停在一杯茶，整场光缓缓退去，黑场结束，自动开始下一轮专注。

**每次不一样的字画。** 墙上挂轴从一组通透格言里随机抽一条，毛笔竖排，挂位、题款、朱印每次都不同。

**延后与跳过都有边界。** 开启「允许跳过休息」会同时关闭「休息时锁定输入」。解除锁定后，休息屏会按设置提供延后 5/10/15 分钟和提前结束。跳过或第三次连续延后时会弹框提醒，确认后下一次休息须燃尽。

**省电降内存。** 休息页与 three.js 渲染循环只在休息前的最后 5 秒挂载，休息结束即卸载，平时专注与待机不跑 WebGL；主窗收起时表盘动画停摆，菜单栏按分钟级对齐刷新，更不常驻 250ms 心跳；四核及以下自动降采样（像素比 1.0、关抗锯齿、贴图 512、粒子减半）。

## 快捷键

| 快捷键 | 作用 |
| --- | --- |
| `⇧⌘S` | 开始专注（待燃时） |
| `⇧⌘P` | 暂停 / 继续专注 |
| `⇧⌘R` | 立即休息 |
| `Esc` | 打开跳过确认框（当前休息允许跳过时），也可关闭确认框 |

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

在 Windows x64 上，使用 nsh v0.6.1、Rust（MSVC 工具链，含 C++ Build Tools 和 Windows SDK）、LLVM 和 Inno Setup 6 打包。确保 `nsh` 和 Inno Setup 的 `ISCC.exe` 在 `PATH` 上，并将 `LIBCLANG_PATH` 指到 LLVM 的 `bin`。用 PowerShell 7 执行打包命令。

```powershell
./scripts/package-windows.ps1
```

安装包和便携 ZIP 输出到 `.nsh/out/windows`。

第二行把 `native/dock_hide.m` 编译成 dylib 注入 App：它通过文件事件监听 `storage.json` 变化，实时切换「只在菜单栏」与「显示在 Dock」，并在休息期间把休息窗抬到最前、安装事件监视器拦截键盘鼠标与切屏；休息结束后卸载事件监视、恢复窗口层级并回到休息前的 App。**每次重新打包都要再跑一次**，否则安装的是一个没有签名过的、Dock 图标永远显示、也没有休息锁的版本。

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

## 发布与更新

推送 `v*` tag 会触发 `.github/workflows/release.yml`。

| runner | 架构 | 归档 |
|---|---|---|
| `macos-15` | arm64 | `xiangzhuan-<version>-macos-aarch64.tar.gz` |
| `macos-15-intel` | x86_64 | `xiangzhuan-<version>-macos-x86_64.tar.gz` |
| `windows-2025` | x86_64 | `xiangzhuan-<version>-windows-x86_64.zip` |

macOS 归档前必须跑 `scripts/install-dock-hide.sh`，否则 OTA 下载到的新外壳会丢掉 Dock 隐藏与休息锁。Release 同时提供 macOS DMG 和 Windows 安装包。壳更新的清单引用各平台归档，热更的清单引用 `com.iwangjie.xiangzhuan-<build>.nshpkg`。

密钥在仓库外 `~/xiangzhuan-keys/`：`root.secret` 离线保管，`release.secret`、`release.cert`、`update.secret` 写进 GitHub secrets（`NSH_RELEASE_KEY`、`NSH_RELEASE_CERT`、`NSH_UPDATE_KEY`）。`nsh.json` 只放公钥：`updater.publicKey` 签 manifest，`updater.packages.roots` 认证签包的 release 密钥。

大文件默认走 GitHub；仓库变量 `MIRROR_PREFIX` 设为 GitHub 镜像前缀（例如 `https://gh-proxy.com/https://github.com/iwangjie/xiangzhuan/releases/download`）后，归档与 nshpkg 的下载 URL 会指向该镜像，清单仍从 GitHub 取。nsh 的更新源只有一个，镜像不可用时没有运行时回退，删掉变量后的下一次发版即恢复 GitHub 直连。

每次发版同时递增 `version` 与 `build`。已安装外壳只接受 build 更高的包，回滚保护会记住已接受过的最高 build。应用启动 3 秒后自动检查（每 6 小时一次）：先找 OTA 包，没有再找整包；找到后静默下载并自动重启，不弹确认。休息进行中会等休息结束后再重启。「设置 → 版本」只显示当前版本与下载进度。

## 结构

```text
nsh.json                 应用名、bundle id、窗口、Info.plist、能力声明
theme.json               中性灰主题 token
App.tsx                  入口：主窗口 + 休息窗口 + 菜单栏 Shell
src/state.ts             计时状态机、设置、统计、持久化
src/Shell.tsx            托盘图标与菜单、应用菜单、快捷键、自适应调度
src/MainWindow.tsx       菜单栏弹窗（计时 / 节奏 / 规则）
src/RestWindows.tsx      多显示器全屏休息屏（WebView 宿主）
src/popover.ts           弹窗吸附到菜单栏图标的位置计算
src/updates.ts           OTA 包 / 整包更新检查与重启
src/ui.tsx               按钮、分段选择、开关、卡片等设计系统
src/login-item.ts        登录时启动（LaunchAgent）
src/shrine-html.ts       休息屏页面（生成物，勿手改）
test/rest-shrine.html    休息屏场景源码（单文件 three.js）
scripts/embed-shrine.py  把场景与 three.js 内联成 src/shrine-html.ts
scripts/install-dock-hide.sh  编译并注入 Dock 隐藏 + 休息锁 dylib
native/dock_hide.m       Dock 显示策略 + 休息锁的 dylib 实现
assets/                  图标、菜单栏模板、three.min.js
```

数据落在 `~/Library/Application Support/com.iwangjie.xiangzhuan/storage.json`。

## 许可

[MIT](LICENSE) © 2026 wangjie

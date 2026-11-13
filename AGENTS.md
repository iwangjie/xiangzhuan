# AGENTS.md

给在本仓库里改代码的 agent。只写「不知道就会改坏」的部分。

## 这个仓库是什么

macOS 菜单栏的专注 / 休息提醒。用 nsh（AppKit 原生视图 + TSX）写宿主，休息屏是一个内联的 three.js 单页场景（WebView）。

- 应用：`nsh.json`（bundle id `com.iwangjie.xiangzhuan`，macOS 14+）
- 入口：`App.tsx` → `src/Shell.tsx`（托盘 / 菜单 / 心跳）+ `src/MainWindow.tsx` + `src/RestWindows.tsx`
- 状态：`src/state.ts`，唯一的事实来源；测试在 `src/state.test.ts`

## 四条硬规则

1. **`src/shrine-html.ts` 是生成物，永远不手改。** 改 `test/rest-shrine.html` 或它引用的素材，然后 `python3 scripts/embed-shrine.py`。脚本会把 Three.js 和离线 3D 数据内联进页面并转义 `</script`。
2. **`storage.json` 的两个 key 是冻结的**（`src/state.ts` 顶部的 `SETTINGS_KEY` / `STATS_KEY`，故意是不可读字符串）。改 key = 老用户的设置和当日统计清零。
3. **`hideDockIcon` 有一条跨语言契约**：`saveSettings()` 往 storage 里另写一个裸 key `hideDockIcon`（`"1"` 隐藏 / `"0"` 显示），`native/dock_hide.m` 每 0.1s 轮询它来切 `NSApplication` 的 activation policy。缺省视为隐藏。改任一侧都要同时改另一侧。
4. **休息锁也有一条跨语言契约**：`startRest / closeRestWindows` 通过 `publishRestBridge()` 写裸 key `restActive` / `restLock`（`"1"` / `"0"`）。`restLock` 为 1 时，`native/dock_hide.m` 把休息窗抬到 `NSScreenSaverWindowLevel`、禁用切屏、用本地事件监视器吞掉键盘和鼠标；`restActive` 归 0 时恢复原窗口层级与 activation 选项，并激活休息前最前面的那个 App。`settings.lockRest` 默认 true，存在设置 blob 里，不要和这两个瞬态 key 混用。

## 常用命令

```sh
nsh check                                  # 类型检查（必须干净）
nsh test                                   # 状态机测试，8 条
nsh package App.tsx --target macos         # 打包到 .nsh/out/香篆.app
sh scripts/install-dock-hide.sh .nsh/out/香篆.app   # 注入 Dock 隐藏 + 休息锁 dylib（每次打包后都要跑）
python3 scripts/embed-shrine.py            # 场景改动后重新生成 shrine-html.ts
```

`nsh run` 会挂在 pty 里，别用它验证；验收用「打包 → 装到 `/Applications` → `open -g`」。

装到本机（会杀掉正在跑的实例）：

```sh
pkill -f '/Applications/香篆.app/Contents/MacOS'
rm -rf /Applications/香篆.app && cp -R .nsh/out/香篆.app /Applications/
xattr -dr com.apple.quarantine /Applications/香篆.app
open -g /Applications/香篆.app
```

## 状态机不变量（改 `state.ts` 前读）

- 阶段：`idle → focus → rest → focus`，另有 `postpone`。`remainingMs` / `totalMs` 是派生量，不要另存一份时间。
- **启动即专注**：`Shell.tsx` 在 `settingsLoaded()` 之后跑一次 `startFocus()`，用 `autoStarted` 保证只跑一次。
- **暂停**只冻结 `focusEnd` 的推进：`paused` + `pausedAt`，恢复时把剩余时间重新挂到 `Date.now()`。暂停中改时长按剩余比例重算。
- **休息收尾由页面驱动**：`finishRest()` 置 `restFinishing` 并挂 `OUTRO_MS = 12000` 的兜底定时器；页面收到 `outro: true` 后立刻开始茶汤收尾，发回 `done` → `finishOutro()`。不要只靠 `remainingMs` 归零触发页面收尾。任何 `startRest / postponeRest / skipRest / stopCycle` 都必须先 `clearFinishTimer()`，否则旧定时器会打断新一轮。
- **休息窗口的关闭有 generation 守卫**（`restGeneration`），`closeRestWindows` 是 420ms 延时 + 代次校验；新增的延时逻辑照抄这个模式。
- **休息结束不显示主窗口**：`finishOutro / skipRest / postponeRest / stopCycle` 都不调用 `show()`；回到主界面只能由用户点托盘或菜单。休息结束时前台交给 `restActive` 归零后的原生逻辑。
- **锁定休息**：`restInputLocked() = phase() === "rest" && settings().lockRest`。锁定时 `canSkipNow` / `canPostponeNow` 恒为 false，`skipRest` / `postponeRest` 因此变成 no-op；页面收到 `locked: true` 后隐藏可交互控件。`lockRest` 默认 true，关掉才回到「可跳过 / 延后」的软休息。
- 每日跳过上限走 `canSkipNow`（`stats().skips < maxDailySkips`），不要另开一条判断。
- `canSkipNow` / `canPostponeNow` / `restForced` 是页面按钮的唯一依据，页面不自己算。

## 休息页契约（宿主 ↔ 场景）

`src/RestWindows.tsx` 每 250ms（或任一字段变化时）向页面 `postMessage` 一条 JSON：

```json
{ "type": "state", "remainingMs": 0, "totalMs": 90000, "progress": 0.4,
  "canPostpone": true, "canSkip": true, "forced": false, "locked": false,
  "outro": false, "active": true, "visitSeed": 1794362490000, "label": "把眼睛从屏幕上移开。",
  "display": "Built-in Retina Display" }
```

页面通过 `window.nsh.postMessage` 回：`ready` / `skip` / `postpone` / `view:<name>` / `done`。

页面侧入口：`window.shrineState(state)`、`window.shrineView(name, silent)`。
**倒计时一律用 `remainingMs`**，不要在页面里自己累加时间；`active` 由 false 变 true 才算新一轮（重抽挂轴、复位壶与杯）。
大肥鱼使用本轮开始时的 `visitSeed` 抽签。宿主发送休息截止时间作为种子，让不同显示器选到同一出场时机和动作顺序。

## 场景维护（`test/rest-shrine.html`）

单文件、无 import、依赖全局 `THREE`。改完必须跑 `embed-shrine.py`。

**已经踩过的坑，不要改回去：**

- 茶流网格必须用手写的管状 `BufferGeometry`（见 `streamGeo` 的 IIFE）。复用 `CylinderGeometry` 会残留几十个没被改写的原始顶点（半径 1m），斟茶时整张桌面被一个茶色巨柱罩住 —— 表现就是「桌子变透明、变茶色」。
- 茶流按真实抛体走：出流只有水平初速，之后只有重力（横向匀速、竖直 `½gt²`），落点由 `t = √(2h/g)` 反推；截面按 `A·v=const` 变细。不要再写「插值到落点 + sin 鼓包」。
- 木桌材质**不能**设 `roughnessMap`。`textures.walnutBump` 当粗糙度图用时整体≈0，会把粗糙度拉到 0，掠射角整张桌子变成镜子。
- 斟茶高度 `pourTeapot.y = tableSurfaceY + 0.072`：再低壶身会压到品杯，再高像悬空。
- 收尾不要推镜头进杯口 —— 会糊成一轮橙色「太阳」。停在「一杯茶」中景，用 `renderer.toneMappingExposure` 缓缓退光收场（`beginSession()` 里必须复位到 1.36）。
- 镜头在 `diving` 阶段绕过轨道夹持（`base = activeView.pos`），并把 `camera.near` 降到 0.004；`phi` 夹持会把镜头顶回桌面上方。
- 曝光 / 光比很脆：`windowLight` 2.2、`hemiLight` 0.72、`ambient` 0.40、`toneMappingExposure` 1.36 是一组调好的值，单改一个会立刻过曝或全黑。
- 低端机分支 `lowEnd` 控制 pixelRatio、阴影贴图和粒子数，新增粒子/贴图请一并接进去。

**页面里已有的时钟与状态**：`outroElapsed`（收尾时间轴）、`burnTarget/burnCurrent`（香长）、`dipApproach`（俯身）、`dusk`（退光）。时间轴：`0–1.6` 移到茶席并提壶，`1.8–3.58` 出汤，`3.3–4.35` 回壶转杯，`4.6–5.9` 俯身，`5.35–6.55` 退光，`7.05` 发 `done`。

## 怎么验证场景

**不要为了看场景在用户机器上弹出全屏休息窗，也不要截用户桌面。** 用浏览器预览：

1. 复制 `test/rest-shrine.html` 为临时预览文件，在最后一个 `})();` 之前注入
   `window.__seek = (s) => { outroStarted = true; outroElapsed = s; diving = true; };`（必须在 IIFE 内，别插到 `})();` 之后）。
2. `python3 -m http.server 8788 --bind 127.0.0.1`（仓库根目录），浏览器打开 `test/xxx-preview.html`。
3. 开场自动演示，或 `window.shrineState({...})` 造状态；`__seek(t)` 后要等约 0.5–0.7s 让镜头阻尼收敛。
4. **只往前 seek**：`#tea-wash` 的 opacity 是 JS 直接写的，往回跳不会复位，会误判成「黑屏 bug」。
5. 预览文件不要提交（`test/*-preview.html` 已在 `.gitignore`）。

## 仓库习惯

- 提交信息用中文，`fix(rest):` / `feat:` + 一句话，正文写清根因。
- 删掉的东西要真删：没有兼容层、没有留着的旧分支、没有 `TODO`。
- 文档只描述现状，不写演进史。

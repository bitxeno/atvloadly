# atvloadly Desktop (mygo + menu bar)

atvloadly 桌面版：用 [mygo](https://github.com/egoist/mygo)（菜单栏壳）+ 复用现有 Go
server 作为本地后端，支持 macOS 12+ 与 Windows 10/11（x64 / arm64）。

## 总体架构

```
┌──────────────────────────────────────────────────────────────┐
│  atvloadly.app / atvloadly.exe（mygo 单二进制 + 嵌入前端）    │
│                                                              │
│  ┌──────────────────────┐      ┌──────────────────────────┐  │
│  │ 菜单栏 / 通知区域    │      │ 内嵌 atvloadly server     │  │
│  │ 浏览器 / 日志 / 退出 │      │ （现有 Fiber web/ 层，    │  │
│  └──────────┬───────────┘      │  复用 internal/* 全部业务 │  │
│             │                  └────────────┬─────────────┘  │
│             │ 默认浏览器 HTTP               │                │
│             ▼                               ▼                │
│   http://127.0.0.1:<port>/      ┌─────────────────────────┐  │
│                                 │ plumesign（随包分发）   │  │
│                                 │ resources 按平台打包，  │  │
│                                 │ 构建时下载并签名        │  │
│                                 └─────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

**设计原则：最大复用，最小包装。** 桌面壳只做三件事：

1. 启动现有 server（复用 `cmd/server` 的引导顺序：config → settings → logger → i18n →
   db → signing → secrets → tasks → device manager），监听 `127.0.0.1:<随机空闲端口>`；
2. 在菜单栏 / 任务栏通知区域驻留图标，菜单提供"打开浏览器"、"打开日志文件"与"退出程序"（文案随界面语言本地化）；
3. 启动后用默认浏览器打开 `http://127.0.0.1:<port>/`；退出时优雅停机。

没有应用主窗口：macOS 不出现在 Dock（`LSUIElement`），Windows 不显示任务栏窗口按钮。
不使用 `mygo.Bind` 的 typed IPC —— 前端仍走 HTTP API + WebSocket（`/ws/install`、
`/ws/pair`、`/ws/login`），代码零改动；不重写 UI，直接复用 Vue 3 前端（`web/static`）。

### 目录结构

```
desktop/
├── README.md                    # 本文件（开发方案）
└── app/
    ├── go.mod                   # 独立 module（mygo 需要 Go 1.27+，见关键决策 #2）
    ├── main.go                  # 桌面壳入口：起 server、菜单栏图标、单实例、打开浏览器
    ├── locales.go               # 内嵌服务端文案（构建时从仓库根 locales/ 复制）
    ├── plumesign.go             # plumesign 资源路径解析（PathResources 优先，PATH 回退）
    ├── mygo.config.ts           # mygo CLI 配置（LSUIElement、buildCommand、resources）
    ├── package.json             # 桌面壳工具链（mygo CLI）
    ├── assets/
    │   ├── tray-darwin.svg      # macOS 菜单栏图标源文件
    │   ├── tray-darwin.png      # 由 SVG 栅格化的黑色透明模板图
    │   └── tray-windows.png     # 通知区域图标，取自 resources/icon.png
    └── resources/
        ├── icon.png             # 桌面应用图标（1024×1024，由 AppIcon.icon 渲染）
        ├── darwin/bin/plumesign          # macOS universal（构建时脚本下载）
        └── windows-amd64/bin/plumesign.exe
```

### 关键决策

| # | 决策 | 理由 |
|---|------|------|
| 1 | **壳 + 内嵌 server，而非 Bind IPC 重写** | `internal/*`（install manager、device manager、WS 登录/配对/进度）全部复用，前端零改动，风险最小 |
| 2 | **独立 `desktop/app/go.mod`** | 根 module 是 go 1.25，mygo 需要 Go 1.27+；隔离后不影响主服务构建与 CI，server 代码通过引用根包复用（见下"复用方式"） |
| 3 | **随机端口 + 127.0.0.1 回环** | 不与用户机器已占用端口冲突；桌面版不对外提供服务 |
| 4 | **plumesign 打进 resources** | mygo 的 platform resources 机制自动按平台打包，`mygo build` 会把 resources 里的可执行文件连带签名，macOS Gatekeeper / Windows SmartScreen 才会放行 |
| 5 | **配置/数据放用户目录** | 桌面应用没有容器 `/data`；默认 `~/Library/Application Support/atvloadly`（macOS）/ `%APPDATA%\atvloadly`（Windows），启动时注入 `server.work_dir` |

> plumesign 上游 release 目前只附了 Linux 包；上游 release workflow 已包含 macOS/Windows
> 构建目标（`plumesign-macos-universal`、`plumesign-windows-*.exe`），按计划补发即可。
> 不阻塞桌面壳开发：本地开发用 PATH 里的 plumesign 回退路径。

## 复用方式（关键技术点）

现有 server 启动逻辑在 `cmd/server/server.go` 的 `action()` 中，是 cli 命令的一部分，
不能直接 import（根 module 与 desktop module 分离）。两种做法：

- **方案 A（推荐）：提取引导函数。** 把 `action()` 里的初始化序列提取为根 module 的
  `internal/app` 或独立 `internal/bootstrap` 包，暴露
  `bootstrap.Run(conf) (net.Listener, stop func(), err)`。`cmd/server` 与
  `desktop/app` 都调用它。桌面壳改动收敛为一次重构 + 一个新包。
  - 难点：`web.Run` 目前自己 `fiber.Listen(addr:port)`。需要加一个接受
    `net.Listener` 的入口（fiber 支持 `app.Listener(ln)`），以便绑定随机端口。
- **方案 B（兜底）：复制引导序列。** desktop module 里写一份精简 bootstrap。
  代码重复，且两处引导会漂移。仅在方案 A 遇到跨 module 编译问题时退回。

跨 module 引用：`desktop/app/go.mod` 中用 `replace` + 相对路径指向仓库根 module，
或用 `go.work`（本地）/ `go.mod` 的 `replace`（CI 与发布构建）。根 module 继续以
go 1.25 语义构建，desktop 只 import 未被 mygo 依赖影响的包。

**locales 内嵌**（与 web 前端同一模式）：服务端文案在根 `locales/`，由根 `main.go`
embed；desktop module 够不到根 main 的 embed，因此构建时由 `buildCommand`
（`npm run sync-locales`）把根 `locales/` 拷进 `desktop/app/locales/`（gitignore），
`locales.go` embed 这份副本，`run()` 里调 `i18n.Init`。根目录结构不动；
目录结构如 web 的 `src → dist` 一样，源在根、副本进包。

菜单文案使用同一份 locales 的 `tray.*` 键：启动时优先使用 Web UI 已保存的语言，
首次启动跟随系统语言；Web UI 切换语言后，`i18n.OnLanguageChanged` 回调会重建菜单。

### 桌面版需要处理的服务端差异

| 差异点 | 现状 | 桌面版处理 |
|--------|------|-----------|
| 设备发现（mDNS） | `device_manager_mdns.go`（`//go:build !linux`）已用纯 Go zeroconf | 无需改动；Windows/macOS 直接可用 |
| lockdown 目录 | `app.LockdownDir()` 已按平台处理 | 无需改动 |
| 配置文件 | `config.yaml`（koanf） | 桌面版不要求用户写 yaml：启动时生成默认配置到用户数据目录 |
| 日志 | lumberjack 文件 + stdout | 写到用户数据目录的 `app.log`，菜单"打开日志文件"用默认编辑器打开 |
| 子进程控制台窗口 | Windows GUI 程序执行控制台子进程会弹出黑窗 | `internal/exec` 统一加 `HideWindow` + `CREATE_NO_WINDOW`，plumesign 全部调用静默 |

## 开发流程

前提：本地 `go` 1.27+（desktop module）、`npm`、plumesign 在 PATH（开发模式回退）。

```sh
cd desktop/app
npm install
npm run dev             # mygo dev：构建并启动桌面壳（驻留菜单栏 + 打开浏览器）
```

### 构建

```sh
cd desktop/app
npm run build           # 开发用：无版本注入
VERSION=1.0.0 COMMIT=abc BUILDDATE="2026-01-01 00:00:00" npm run build:release
# build:release 生成 version_gen.go（gitignore）注入 internal/app/build 的
# Version/Commit/BuildDate/Mode —— mygo build 自组 -ldflags 会覆盖 GOFLAGS
# 传入的 -X，所以走生成文件而非 ldflags（与 xgo -ldflags 注入等效）。
```

签名发布需要：macOS Developer ID + notarytool 凭据；Windows 代码签名证书。
`mygo.config.ts` 中按环境变量注入（`MACOS_SIGNING_IDENTITY` 等），本地构建自动
ad-hoc 签名。

实现备注：

- **纯 Go 构建**：bom 已 vendor 进 `pkg/` 并把 lzfse 换成 purego 的
  `bitxeno/lzfse-go`，全依赖无 cgo，`CGO_ENABLED=0` 即可编译任意平台目标；
  任意 runner 都能交叉出全部平台包。
- **DMG 与外置盘**：`hdiutil attach` 在外置 USB 盘（APFS）上挂载被拒；DMG 创建需
  在内置盘工作区运行，或用 `-skip-dmg` 只出 .app。
- **mygo generate 副作用**：`mygo generate` 会运行主程序来写 TS client；本壳无
  Bind，main 里检测 `MYGO_GENERATE` 直接返回，避免 generate 时把服务跑起来。

### plumesign 集成

- **下载**：`scripts/fetch-plumesign.sh`（构建前执行）从 PlumeImpactor release 下载
  `plumesign-macos-universal.tar.gz`（解包后同时供 darwin-arm64 与 darwin-amd64，
  mygo build 自动 lipo）与 `plumesign-windows-x86_64.exe`，放进
  `desktop/app/resources/<platform>/bin/`（.gitignore，不入库）。上游未发布的资产
  （当前缺 windows-arm64）逐项警告并继续。
- **路径解析**：现有 `internal/exec`、manager 代码用 `"plumesign"` 裸命令名，桌面壳在
  进程启动时把 `PathResources/bin` 加入 `PATH` 环境变量（子进程继承），业务代码零改动。
  已在 `env -i` 干净环境下端到端验证：打包 app 的 `check/afc` 接口成功执行 bundle 内
  的 plumesign 2.7.1-patch.5；未打包（go run）回退系统 PATH。

## 开发里程碑

| 里程碑 | 内容 | 验收 |
|--------|------|------|
| M1 桌面壳跑通 ✅ | 独立 go.mod、bootstrap 提取、内嵌 server + 默认浏览器加载回环地址 | macOS 打包 app 启动：浏览器显示完整前端，mDNS 发现设备，回环 200 |
| M2 图标 + plumesign + 单实例 ✅ | Icon Composer 图标（Liquid Glass）、plumesign 随包分发、单实例锁 | 菜单栏显示图标；bundle 内签名 plumesign 进 PATH；二次启动在已有实例中打开浏览器 |
| M3 发布打磨 ✅ | 版本号注入（version_gen.go）、CI（desktop-release.yml：macOS universal 签名公证 + Windows NSIS，desktop-v* tag 触发） | `npm run build:release` 后 `/api/version` 报告注入版本；CI 待真实 tag 验证 |
| M4 菜单栏常驻 ✅ | 去掉主窗口，macOS `LSUIElement` + Windows 通知区域图标；单击图标显示"打开浏览器 / 打开日志文件 / 退出程序" | 双击运行后自动打开浏览器且无主窗口；退出菜单停止内嵌 server 并结束进程 |
| M5 Windows 打磨 ✅ | Windows 应用/通知区域图标取自 `AppIcon.icon` 的渲染；plumesign 子进程不再弹出控制台窗口 | 任务栏与通知区域显示 App 图标；配对、安装等操作全程无黑窗 |

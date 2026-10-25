# atvloadly Desktop (mygo + menu bar)

The desktop build of atvloadly: a [mygo](https://github.com/egoist/mygo) menu bar
shell around the existing Go server, which it reuses as a local backend. Supports
macOS 12+ and Windows 10/11 (x64 / arm64).

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│  atvloadly.app / atvloadly.exe (one mygo binary + embedded frontend)         │
│ ┌────────────────────────────┐      ┌──────────────────────────────────────┐ │
│ │ Menu bar / tray            │      │ embedded atvloadly server            │ │
│ │ Browser / Log / Quit       │      │ (existing Fiber web/ layer,          │ │
│ └──────────────┬─────────────┘      │ reuses all of internal/*)            │ │
│                │                    └──────────────────┬───────────────────┘ │
│                │ default browser HTTP                  │                     │
│                ▼                                       ▼                     │
│  http://127.0.0.1:<port>/           ┌──────────────────────────────────────┐ │
│                                     │ plumesign (shipped in the app)       │ │
│                                     │ resources packed per platform,       │ │
│                                     │ downloaded and signed at build       │ │
│                                     └──────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Design principle: maximum reuse, minimum wrapping.** The shell does three things:

1. Starts the existing server (reusing the boot order of `cmd/server`: config → settings
   → logger → i18n → db → signing → secrets → tasks → device manager) on
   `127.0.0.1:<random free port>`;
2. Keeps an icon in the menu bar / notification area, with a menu holding "Open in
   Browser", "Open Log File" and "Quit" (localized in the UI language);
3. Opens `http://127.0.0.1:<port>/` in the default browser once started, and shuts
   down gracefully on quit.

There is no main window: the app stays out of the Dock on macOS (`LSUIElement`) and
shows no taskbar button on Windows. It does not use `mygo.Bind` typed IPC — the
frontend keeps using the HTTP API and WebSockets (`/ws/install`, `/ws/pair`,
`/ws/login`), so no code changes there; the Vue 3 frontend in `web/static` is reused
as is instead of being rewritten.

### Layout

```
desktop/
├── README.md                    # this file (the design)
└── app/
    ├── go.mod                   # separate module (mygo needs Go 1.27+, see decision #2)
    ├── main.go                  # shell entry: starts the server, tray icon, single instance, opens the browser
    ├── locales.go               # embeds the server catalogs (copied from the root locales/ at build time)
    ├── plumesign.go             # resolves the plumesign resource path (PathResources first, PATH fallback)
    ├── mygo.config.ts           # mygo CLI config (LSUIElement, macOS signing, resources)
    ├── package.json             # desktop toolchain (mygo CLI)
    ├── assets/
    │   ├── tray-darwin.svg      # source of the macOS menu bar icon
    │   ├── tray-darwin.png      # black-on-transparent template rasterized from the SVG
    │   └── tray-windows.png     # notification area icon, taken from resources/icon.png
    └── resources/
        ├── icon.png             # app icon (1024×1024, rendered from AppIcon.icon)
        ├── darwin/bin/plumesign          # macOS universal (downloaded by the build script)
        └── windows-amd64/bin/plumesign.exe
```

### Key decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | **Shell + embedded server, not a Bind IPC rewrite** | `internal/*` (install manager, device manager, WS login/pair/progress) is reused whole, the frontend needs no changes, and the risk stays low |
| 2 | **A separate `desktop/app/go.mod`** | The root module is go 1.25 and mygo needs Go 1.27+; isolating it keeps the main service build and CI untouched, while server code is reused by importing the root packages (see "How the server is reused") |
| 3 | **Random port + 127.0.0.1 loopback** | Never collides with a port the user already uses; the desktop build serves nothing to the network |
| 4 | **plumesign packed into resources** | mygo's platform resources mechanism packages it per platform, and `mygo build` signs the executables along with the app, which is what lets macOS Gatekeeper / Windows SmartScreen through |
| 5 | **Config and data in the user directory** | A desktop app has no container `/data`; defaults to `~/Library/Application Support/atvloadly` (macOS) / `%APPDATA%\atvloadly` (Windows), with `server.work_dir` injected at startup |

> The upstream plumesign release currently ships Linux packages only; its release
> workflow already has the macOS/Windows targets (`plumesign-macos-universal`,
> `plumesign-windows-*.exe`), so they only need to be published. This does not block
> the shell: local development falls back to plumesign on the PATH.

## How the server is reused (the key technique)

The existing startup logic lives in `action()` in `cmd/server/server.go`, part of a CLI
command, so it cannot be imported directly (the root and desktop modules are separate).
Two approaches:

- **Option A (chosen): extract a bootstrap function.** The initialization sequence in
  `action()` became the root module's `internal/bootstrap` package, exposing
  `bootstrap.Run(opts) (addr string, stop func(), err error)`. Both `cmd/server` and
  `desktop/app` call it, so the shell's footprint is one refactor plus one new package.
  - The tricky part: `web.Run` used to call `fiber.Listen(addr:port)` itself. It now has
    an entry point that takes a `net.Listener` (fiber supports `app.Listener(ln)`), so
    the shell can bind a random port.
- **Option B (fallback): copy the boot sequence.** Write a slim bootstrap inside the
  desktop module. It duplicates code and the two boot paths drift apart; only fall back
  to it if Option A hits a cross-module compile problem.

Cross-module reference: `desktop/app/go.mod` points at the root module with a `replace`
and a relative path. The root module keeps building with go 1.25 semantics, and the
desktop app imports only packages that mygo's dependencies do not affect.

**Embedded locales** (the same pattern as the web frontend): the server catalogs live in
the root `locales/`, embedded by the root `main.go`. The desktop module cannot reach that
embed, so the npm scripts run `sync-locales` to copy the root `locales/` into
`desktop/app/locales/` (gitignored), `locales.go` embeds that copy, and `run()` calls
`i18n.Init`. The root layout is unchanged; like web's `src → dist`, the source stays at
the root and a copy goes into the bundle.

The copy has to happen before `mygo build` starts, so it lives in the npm scripts
(`build`, `build:release`, `dev`) rather than in `buildCommand`: mygo generates the
TypeScript client with a `go build` of its own and only runs `buildCommand` afterwards,
which is too late for `//go:embed locales` on a clean checkout.

The menu uses the `tray.*` keys of the same catalogs: at startup it prefers the language
saved by the web UI and follows the system language on first launch; when the language
changes in the web UI, the `i18n.OnLanguageChanged` callback rebuilds the menu.

### Server differences the desktop build handles

| Difference | Current state | Desktop handling |
|------------|---------------|------------------|
| Device discovery (mDNS) | `device_manager_mdns.go` (`//go:build !linux`) already uses pure-Go zeroconf | No change needed; works on Windows/macOS directly |
| Lockdown directory | `app.LockdownDir()` already handles each platform | No change needed |
| Config file | `config.yaml` (koanf) | The desktop build does not make users write YAML: it generates a default config into the user data directory at startup |
| Logging | lumberjack file + stdout | Written to `app.log` in the user data directory; "Open Log File" opens it in the default editor |
| Console windows of child processes | A Windows GUI program running a console child pops up a black window | `internal/exec` sets `HideWindow` + `CREATE_NO_WINDOW` everywhere, so every plumesign call is silent |

## Development

Prerequisites: a local `go` 1.27+ (for the desktop module), `npm`, and plumesign on the
PATH (the development fallback).

```sh
cd desktop/app
npm install
npm run dev             # mygo dev: builds and starts the shell (menu bar + opens the browser)
```

### Build

```sh
cd desktop/app
npm run build           # development: no version injection
VERSION=1.0.0 COMMIT=abc BUILDDATE="2026-01-01 00:00:00" npm run build:release
# build:release generates version_gen.go (gitignored) to inject Version/Commit/
# BuildDate/Mode into internal/app/build — mygo build composes its own -ldflags,
# which would override -X passed through GOFLAGS, so this goes through a
# generated file instead (equivalent to xgo's -ldflags injection).
```

A signed release needs a macOS Developer ID plus notarytool credentials, and a Windows
code signing certificate. `mygo.config.ts` injects them from environment variables
(`MACOS_SIGNING_IDENTITY` and friends); local builds sign ad hoc.

### macOS install (the DMG is unsigned)

The DMG is unsigned. After dragging `atvloadly.app` into Applications, run once:

```sh
sudo xattr -dr com.apple.quarantine /Applications/atvloadly.app
```

The recursive `-dr` matters: the bundled `plumesign` carries the quarantine flag too, and
the system kills it at run time if it is left behind.

Implementation notes:

- **Pure-Go build**: bom is vendored into `pkg/` and lzfse goes through the purego-based
  `bitxeno/lzfse-go`, so no dependency needs cgo and `CGO_ENABLED=0` compiles every
  target; any runner can cross-build all platform packages.
- **DMG and external drives**: `hdiutil attach` refuses to mount on an external USB
  (APFS) drive; create the DMG from a workspace on the internal disk, or use `-skip-dmg`
  to produce only the `.app`.
- **`mygo generate` side effect**: `mygo generate` runs the main program to write the TS
  client; this shell binds nothing, so `main` returns early when it sees `MYGO_GENERATE`
  and the service never starts during generate.

### plumesign integration

- **Download**: `scripts/fetch-plumesign.sh` (run before the build) downloads
  `plumesign-macos-universal.tar.gz` (unpacked for both darwin-arm64 and darwin-amd64,
  which `mygo build` lipos together) and `plumesign-windows-x86_64.exe` from the
  PlumeImpactor release into `desktop/app/resources/<platform>/bin/` (gitignored, not
  committed). Assets upstream has not published yet (currently windows-arm64) warn
  individually and the build continues.
- **Path resolution**: the existing `internal/exec` and manager code call the bare
  `"plumesign"` command; at startup the shell prepends `PathResources/bin` to the `PATH`
  environment variable (inherited by child processes), so the business code needs no
  changes. Verified end to end under a clean `env -i`: the packaged app's `check/afc`
  endpoint ran the bundled plumesign 2.7.1-patch.5; unpackaged (`go run`) falls back to
  the system PATH.

## Milestones

| Milestone | Scope | Acceptance |
|-----------|-------|------------|
| M1 shell runs ✅ | Separate go.mod, extracted bootstrap, embedded server + default browser on the loopback address | Packaged macOS app starts: the browser shows the full frontend, mDNS finds devices, loopback answers 200 |
| M2 icon + plumesign + single instance ✅ | Icon Composer icon (Liquid Glass), plumesign shipped in the bundle, single-instance lock | Menu bar shows the icon; the signed plumesign inside the bundle is on the PATH; a second launch opens the browser in the running instance |
| M3 release polish ✅ | Version injection (version_gen.go), CI (`release.yml`: the desktop builds run alongside the server build on a `v*` tag and publish into the same Release — macOS universal signing + notarization, Windows NSIS) | After `npm run build:release`, `/api/version` reports the injected version; CI awaits a real tag to verify |
| M4 menu bar resident ✅ | Drop the main window, macOS `LSUIElement` + Windows notification area icon; clicking the icon shows "Open in Browser / Open Log File / Quit" | Double-clicking opens the browser with no main window; Quit stops the embedded server and ends the process |
| M5 Windows polish ✅ | The Windows app/notification area icon comes from the rendered `AppIcon.icon`; plumesign child processes no longer pop a console window | Taskbar and notification area show the app icon; pairing, installing and the rest never flash a black window |

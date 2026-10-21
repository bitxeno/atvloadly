# macOS

Run atvloadly natively on Apple Silicon or Intel Macs. Device discovery uses
mDNS on the Mac's local network, and USB pairing uses Apple's system usbmuxd.
Docker, Avahi, D-Bus and usbmuxd2 are not required.

[Build and start](#build-and-start) · [Pair an Apple TV](#pair-an-apple-tv) ·
[Data and configuration](#data-and-configuration) ·
[Troubleshooting](#troubleshooting)

## Requirements

Build on the Mac that will run the service. Install:

- Xcode Command Line Tools: `xcode-select --install`.
- Go 1.25 or newer.
- Node.js 24 or newer, with npm.
- The current stable Rust toolchain, with Cargo, for the signing engine.

## Build and start

Run the following commands from the atvloadly repository root.

### 1. Build atvloadly

Build the web UI before the Go binary, which embeds the UI assets:

```bash
mkdir -p build
(cd web/static && npm ci && npm run build)
go build -o build/atvloadly .
```

### 2. Build the signing engine

Build `plumesign` from the default branch of the
[PlumeImpactor fork](https://github.com/bitxeno/PlumeImpactor). Enable
`omnisette/aos-kit` to use macOS's native Anisette provider:

```bash
git clone --depth 1 https://github.com/bitxeno/PlumeImpactor.git build/PlumeImpactor
(cd build/PlumeImpactor && cargo build --release --locked -p plumesign -p omnisette --features omnisette/aos-kit)
cp build/PlumeImpactor/target/release/plumesign build/
```

The first build downloads and compiles Rust dependencies and may take several
minutes. `--locked` uses the dependency versions in the checked-out source's
`Cargo.lock`.

If `build/PlumeImpactor` already exists, use that checkout after updating it to
the source you want to build, then run the Cargo and copy commands above.

atvloadly looks for `plumesign` beside its own executable before searching
`PATH`. Keep the two binaries together; a generic Impactor GUI installation
does not supply the CLI used here.

### 3. Start the service

```bash
cp doc/config.macos.yaml.example build/config.yaml
./build/atvloadly server --config ./build/config.yaml
```

Open [http://127.0.0.1:9000](http://127.0.0.1:9000). The service runs in the
foreground; stop it with **Ctrl+C**. Automatic refresh requires the Mac to stay
awake and the service to remain running and able to reach the Apple TV.

Build output is stored in the Git-ignored `build/` directory. To move the
service, copy `atvloadly`, `plumesign` and `config.yaml` together, then start it
with the new config path. Application data stays in the directories listed
below.

## Pair an Apple TV

1. Connect the Mac and Apple TV to the same LAN. Allow local network access for
   the terminal or service when macOS asks.
2. On Apple TV, open **Settings → Remotes and Devices → Remote App and Devices**.
3. In the web UI, select the discovered Apple TV and enter its pairing code.
4. Install a tvOS IPA using an Apple ID, or follow the
   [external certificate guide](./external-certificate.md) to use a P12 and tvOS
   provisioning profile.

### Devices paired over USB

For a device that supports USB pairing, connect it to the Mac, unlock it and
choose **Trust** when asked. Finder or Xcode can start this prompt. Enable
Wi-Fi sync in Finder so the device can appear as a paired Lockdown device on
the same network after disconnecting the cable.

atvloadly reads these pair records through the system usbmuxd without `sudo`.
It does not modify or delete them. Manage USB trust and Wi-Fi sync through
Finder or the device's settings; do not replace or restart Apple's usbmuxd.

## Data and configuration

The startup command above loads `build/config.yaml`. Without `--config`, the
service looks for `~/Library/Application Support/atvloadly/config.yaml`.

| Data | Default location |
| --- | --- |
| Database, settings, logs, IPA files and encryption keys | `~/Library/Application Support/atvloadly/` |
| Signing engine accounts and certificates | `~/.config/PlumeImpactor/` |
| Remote pairing records | `~/.config/PlumeImpactor/pairing_files/` |

- `server.work_dir` changes the service's data directory, including the default
  database, log and encryption key locations. It does not move engine state or
  change which config file is loaded.
- `XDG_CONFIG_HOME`, when set, replaces `~/.config` for the signing engine and
  the service's account and remote pairing lookups.
- Use absolute paths in config files. Back up both the service data directory
  and the engine directory, including encryption keys. Renaming the binary or
  using `go run` does not change the service's default macOS data directory.

### Access from other devices

The example config listens on `127.0.0.1`. To make the web UI available on the
LAN, set `server.listen_addr` to `0.0.0.0`, restart the service and allow incoming
connections when macOS asks. Connect to the Mac's LAN address on port `9000`.
atvloadly has no authentication, so only expose it on a trusted network.

## Troubleshooting

| Symptom | What to check |
| --- | --- |
| Startup reports that `plumesign` is missing | Place the executable beside `atvloadly` or on `PATH`. |
| No Apple TV appears | Check that both devices are on the same LAN, local network permission is allowed, and the Apple TV pairing screen is open. Check VPN routing and multicast filtering. |
| Discovery stops after changing Wi-Fi or waking the Mac | Network interfaces reload every 30 seconds. Wait briefly or rescan from the web UI. |
| A USB-paired device does not appear over Wi-Fi | Check USB trust, Finder's Wi-Fi sync setting and that the device is on the same LAN. |
| The UI is unreachable from another device | Check the listen address, port and macOS firewall permissions. |
| Automatic refresh does not run | Keep the Mac awake, the service running and the Apple TV reachable. Apps signed with an imported certificate are not refreshed automatically. |

CoreADI APK updates are Linux-only. The macOS build uses system Anisette and
does not need Android `.so` files. External certificate signing isolates both
`HOME` and `XDG_CONFIG_HOME` from the shared Apple ID session.

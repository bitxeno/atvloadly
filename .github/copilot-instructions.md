# atvloadly Development Guide

## Architecture Overview

atvloadly is a web service for sideloading apps on Apple TV, built with Go + Fiber web framework + Vue.js frontend.

### Core Components

- **PlumeImpactor Integration**: Uses external `plumesign` CLI tool for IPA signing and installation
  - Command location: `/usr/bin/plumesign` (Docker environment)
  - Account data storage: `~/.config/PlumeImpactor/accounts.json`
  - Key invocation: [install_manager.go](internal/manager/install_manager.go) line 72

- **Device Manager**: [internal/manager/](internal/manager/) 
  - Discovers Apple TV devices using Avahi (Linux) or mDNS
  - Communicates with iOS/tvOS devices via `gidevice` library
  - Supports device pairing, developer image mounting, AFC service checks

- **WebSocket Real-time Communication**: [internal/service/websocket.go](internal/service/websocket.go)
  - `/ws/install`: IPA installation progress streaming
  - `/ws/pair`: Device pairing workflow
  - `/ws/tty`: Terminal access (for debugging)

- **Scheduled Tasks**: [internal/task/](internal/task/)
  - Auto-refresh installed apps (prevents 7-day signature expiration)
  - Uses `robfig/cron` scheduler

## Platform Limitations

**Linux/OpenWrt ONLY**, macOS/Windows not supported:
- Depends on `avahi-daemon` service for device discovery
- Requires `usbmuxd2` for iOS device communication
- Docker image based on Ubuntu 22.04 ([Dockerfile](Dockerfile))

## Development Workflow

### Local Execution
```bash
go run main.go server --config ./doc/config.yaml.example --debug
```

### Build
```bash
# Single platform build
go build -o atvloadly main.go

# Multi-architecture build (see build/ directory)
GOOS=linux GOARCH=amd64 go build -o build/atvloadly-linux-amd64
GOOS=linux GOARCH=arm64 go build -o build/atvloadly-linux-arm64
```

### Docker Deployment
```bash
docker compose up -d
# Must mount: /var/run/dbus and /var/run/avahi-daemon
```

## Code Conventions

### Configuration Management
- Uses `koanf` to load YAML config ([internal/app/config.go](internal/app/config.go))
- Runtime settings stored in JSON: `settings.json` ([internal/app/settings.go](internal/app/settings.go))
- Default config directory: `~/.config` or `DataDir` (`/data` in Docker)

### API Response Format
All APIs return uniform JSON wrapper ([web/api_result.go](web/api_result.go)):
```go
return c.Status(http.StatusOK).JSON(apiSuccess(data))
return c.Status(http.StatusOK).JSON(apiError("error message"))
```

### Logging
- Uses `rs/zerolog` ([internal/log/](internal/log/))
- Dual output to file and console
- Log levels: `info` (default), `debug`, `trace`

### Database
- SQLite + GORM ([internal/db/](internal/db/))
- Primary model: [model/installed_app.go](internal/model/installed_app.go)
- Auto-migration at [bootstrap.go](internal/app/bootstrap.go) line 91

### i18n
- Uses `go-i18n/v2` ([internal/i18n/](internal/i18n/))
- Translation files: [internal/i18n/locales/](internal/i18n/locales/)
- User language preference synced via `/api/lang/sync`

### Comments
- **Mandatory**: All code comments and documentation MUST be written in English, even if the user asks in another language.

## Key Integration Points

### External Process Invocation
```go
// Install IPA (requires Apple ID credentials)
cmd := exec.CommandContext(ctx, "plumesign", "sign", "--apple-id", 
    "--register-and-install", "--udid", udid, "-u", account, "-p", ipaPath)

// Check AFC service
cmd := exec.Command("plumesign", "check", "afc", "--udid", udid)
```

### Device Discovery
- Linux: Communicates with `avahi-daemon` via D-Bus ([device_manager_avahi.go](internal/manager/device_manager_avahi.go))
- Fallback: Uses `grandcat/zeroconf` mDNS library ([device_manager_mdns.go](internal/manager/device_manager_mdns.go))

### IPA Parsing
- Uses `iineva/ipa-server` library to extract metadata ([internal/ipa/](internal/ipa/))
- Reads `Info.plist`, extracts icon, obtains Bundle ID

## Common Tasks

### Adding New API Endpoint
Register under `api` group in [web/router.go](web/router.go):
```go
api.Post("/your-endpoint", func(c *fiber.Ctx) error {
    return c.Status(http.StatusOK).JSON(apiSuccess(result))
})
```

### Modifying Device Detection Logic
Refer to `ReloadDevices()` method in [device_manager.go](internal/manager/device_manager.go)

### Adjusting Auto-Refresh Strategy
Edit [internal/task/task.go](internal/task/task.go), modify cron expression parsing logic

## Debugging Tips

- Use `--debug` flag to see verbose logs and database queries
- Access container bash via `/ws/tty` WebSocket (dev environment only)
- Check service status: `GET /api/service/status` (returns avahi-daemon and usbmuxd status)
- View installation logs: `/apps/:id/log` returns task log file

## Security Considerations

- **NEVER enable TTY WebSocket in production** ([web/router.go](web/router.go) line 42)
- Apple ID passwords passed as command-line arguments to `plumesign`, ensure sensitive info is filtered in logs
- Container requires `--privileged` mode for USB device access (usbmuxd requirement)

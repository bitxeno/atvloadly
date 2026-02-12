# Agent Guidelines for atvloadly

> This file provides instructions for AI coding agents working in this repository.

## Project Overview

atvloadly is a Go-based web service for sideloading apps onto Apple TV devices. It uses:
- **Backend**: Go 1.18+ with Fiber web framework
- **Frontend**: Vue.js 3 + Vite + TailwindCSS
- **Database**: SQLite with GORM
- **External Tools**: PlumeImpactor (`plumesign` CLI) for IPA signing

## Build Commands

```bash
# Build the Go binary
go build -o atvloadly main.go

# Build for specific platform
GOOS=linux GOARCH=amd64 go build -o build/atvloadly-linux-amd64 main.go

# Run locally with debug mode
go run main.go server --config ./doc/config.yaml.example --debug

# Run with hot reload (requires air)
air

# Build frontend (required before Go build)
cd ./web/static && npm install && npm run build
```

## Test Commands

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.txt -covermode=atomic ./...

# Run a single test
go test -run TestCommand_CombinedOutput ./internal/exec/

# Run tests for specific package
go test ./internal/service/
```

## Lint Commands

```bash
# Run golangci-lint (used in CI)
golangci-lint run --timeout=5m

# Format Go code
gofmt -w .

# Tidy modules
go mod tidy
```

## Code Style Guidelines

### Import Order
1. Standard library imports
2. Third-party imports (alphabetical)
3. Local imports (`github.com/bitxeno/atvloadly/...`)

### Error Handling
- Use `pkg/errors` or `go-errors/errors` for stack traces
- Wrap errors with context: `errors.Wrap(err, "context")`
- Check error handling in tests with `t.Errorf()`

### Naming Conventions
- **Exported**: PascalCase (e.g., `InstallManager`, `GetDevices()`)
- **Unexported**: camelCase (e.g., `quietMode`, `outputStdout`)
- **Constants**: CamelCase or PascalCase (e.g., `AppName`, `ErrCommandTimeout`)
- **Interfaces**: Noun with -er suffix (e.g., `Writer`, `Handler`)

### API Responses
All HTTP responses use consistent JSON wrappers:
```go
// Success
c.Status(http.StatusOK).JSON(apiSuccess(data))

// Error
c.Status(http.StatusOK).JSON(apiError("error message"))
```

### Comments
- **MANDATORY**: All code comments must be written in English
- Use Go-style comments: `// Comment text`
- Document exported functions, types, and packages

### Logging
- Use `rs/zerolog` via `internal/log/` package
- Log levels: `info` (default), `debug`, `trace`
- Enable debug with `--debug` flag

### Database (GORM)
- Models in `internal/model/`
- Auto-migration in `internal/app/bootstrap.go`
- Use struct tags for JSON and DB: `json:"field" gorm:"column:field"`

### Configuration
- YAML config via `koanf`: `internal/app/config.go`
- Runtime settings in JSON: `internal/app/settings.go`
- Default data directory: `~/.config` or `/data` in Docker

### WebSocket Endpoints
- `/ws/install` - Installation progress
- `/ws/pair` - Device pairing
- `/ws/login` - Login workflow
- `/ws/tty` - Terminal access (dev only!)

### Key File Locations
- Router: `web/router.go`
- API result helpers: `web/api_result.go`
- Device manager: `internal/manager/device_manager.go`
- Install manager: `internal/manager/install_manager.go`
- Models: `internal/model/`

## Security Considerations

- **NEVER** enable TTY WebSocket (`/ws/tty`) in production
- Filter sensitive data (passwords) from logs
- Container requires `--privileged` for USB access

## Git Commit Messages

**MANDATORY**: Use Conventional Commits specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only changes
- `style`: Code style changes (formatting, semicolons, etc.)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or correcting tests
- `chore`: Build process or auxiliary tool changes

### Examples
```
feat(device): add wireless device scanning support

fix(install): resolve timeout issue during IPA installation

docs(readme): update installation instructions
```

## Platform Limitations

- **Linux/OpenWrt ONLY** - macOS/Windows not supported
- Requires `avahi-daemon` for device discovery
- Requires `usbmuxd2` for iOS device communication

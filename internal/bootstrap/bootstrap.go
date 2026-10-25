// Package bootstrap starts the atvloadly service: the initialization sequence
// of the server command, without the command line. The server command and the
// desktop app both run it, and the desktop app serves the web app on the
// listener it returns.
package bootstrap

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/secret"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/bitxeno/atvloadly/internal/task"
	"github.com/bitxeno/atvloadly/web"
)

// Options configures a service run. Zero fields fall back to the same
// defaults the server command uses.
type Options struct {
	// ConfigFile is the yaml file to load; empty means the default location.
	ConfigFile string
	// Debug turns on debug logging and sql logging.
	Debug bool
	// Trace turns on trace logging and sql logging; implies Debug.
	Trace bool
	// Port overrides conf.Server.Port; zero keeps the configured port.
	// Port -1 binds a random free port instead.
	Port int
	// DataDir overrides conf.Server.DataDir, the directory holding the
	// database, settings, keys and logs; empty keeps the configured dir.
	DataDir string
}

// Run initializes the service and serves the web app. It returns after the
// listener is bound (the service keeps running in the background); stop shuts
// the listener down and returns once the web app has stopped.
func Run(opts Options) (addr string, stop func(), err error) {
	conf, err := app.InitConfig(opts.ConfigFile, opts.Debug)
	if err != nil {
		return "", nil, err
	}
	if opts.DataDir != "" {
		conf.Server.DataDir = opts.DataDir
	}
	settings, err := app.InitSettings(conf, opts.Debug)
	if err != nil {
		return "", nil, err
	}

	if opts.Trace {
		conf.Log.Level = "trace"
		conf.Db.Debug = true
	} else if opts.Debug {
		conf.Log.Level = "debug"
		conf.Db.Debug = true
	}
	if err := app.InitLogger(conf); err != nil {
		return "", nil, err
	}

	app.InitLanguage(settings.App.Language)

	if err := app.InitDb(conf); err != nil {
		return "", nil, err
	}
	initSigning(conf)
	initSecrets(conf)
	// Staged uploads and downloads abandoned by the install page; the task
	// scheduler repeats this every hour.
	if _, err := service.RemoveStaleTempFiles(); err != nil {
		log.Err(err).Msg("Failed to remove stale staged files")
	}

	// start jobs
	_ = task.ScheduleRefreshApps()
	manager.StartDeviceManager()

	port := conf.Server.Port
	if opts.Port > 0 {
		port = opts.Port
	}
	addr = fmt.Sprintf("%s:%d", conf.Server.ListenAddr, port)
	if opts.Port < 0 {
		// A random free port on loopback: the desktop app is not a network
		// service and must not collide with anything already listening.
		addr = "127.0.0.1:0"
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return "", nil, err
	}
	serving := make(chan struct{})
	go func() {
		defer close(serving)
		if err := web.RunListener(listener); err != nil {
			log.Error(err.Error())
		}
	}()

	stopped := make(chan struct{})
	go func() {
		<-stopped
		manager.StopDeviceManager()
		_ = listener.Close()
		<-serving
	}()

	return listener.Addr().String(), func() { close(stopped) }, nil
}

// initSigning configures the external certificate signing mode and removes
// the signing workspaces left behind by a previous crash. Failures are logged
// and do not prevent the server from starting.
func initSigning(conf *app.Configuration) {
	signing.Configure(conf.Signing.KeyFile, filepath.Join(conf.Server.DataDir, "signing-work"))
	removed, err := signing.RecoverWorkspaces()
	if err != nil {
		log.Err(err).Msg("Failed to recover signing workspaces")
		return
	}
	if removed > 0 {
		log.Infof("Removed %d stale signing workspace(s)", removed)
	}
}

// initSecrets configures the dedicated secret store. A stored GitHub token
// that cannot be opened only disables authenticated GitHub calls.
func initSecrets(conf *app.Configuration) {
	secret.Configure(conf.Secret.KeyFile)
	if err := app.CheckGitHubToken(); err != nil {
		log.Warnf("Stored GitHub token cannot be unlocked: %v", err)
	}
}

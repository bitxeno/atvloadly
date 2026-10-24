// The atvloadly desktop app: a mygo shell around the atvloadly server.
// The shell starts the service on a loopback port of its own and stays in
// the menu bar (macOS) or notification area (Windows): choosing "Open in
// Browser" opens the web frontend in the default browser, "Open Log File"
// shows the service log, "Quit" stops the service and exits. There is no
// app window.
package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/bootstrap"
	"github.com/bitxeno/atvloadly/internal/i18n"
	"github.com/egoist/mygo"
)

const appName = "atvloadly"

// trayDarwinIcon is the menu bar mark, rasterized from tray-darwin.svg at
// 144x144: black on transparent, which macOS tints as a template image.
//
//go:embed assets/tray-darwin.png
var trayDarwinIcon []byte

// trayWindowsIcon is the app icon (the rendering of AppIcon.icon that
// resources/icon.png holds) at 64x64: the notification area shows a colored
// icon, not a template one.
//
//go:embed assets/tray-windows.png
var trayWindowsIcon []byte

func main() {
	if os.Getenv("MYGO_GENERATE") != "" {
		// mygo generate runs the app only to write the client of the
		// services bound with mygo.Bind; this app binds none and the
		// service must not start for it.
		return
	}
	if err := run(); err != nil {
		fatal(err)
	}
}

func run() error {
	// A second launch would start a second service on the same data
	// directory; the first instance opens the browser again instead.
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}

	initI18n()

	// The service shares the desktop app's identity: data lives in the app's
	// user data directory instead of ~/.config/<binary name>.
	app.SetAppDataDir(dataDir())

	// Plumesign ships inside the app bundle; make it the plumesign the
	// managers exec.
	if dir := plumesignBinDir(); dir != "" {
		// Setenv on the process environment never fails with a non-empty
		// value; ignore the error for the errcheck of go vet.
		_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	url, stop, err := startService()
	if err != nil {
		return err
	}

	// Quit stops the service; a crash exit leaves it to the process exit.
	defer func() { stop() }()

	// The service chooses the log file while starting: the data directory's
	// app.log unless config.yaml says otherwise.
	logPath := app.Config.Log.LogFile

	mygo.App.SetName(appName)
	mygo.App.SetVersion(app.Version.Version)
	// Menu bar apps keep running without windows; on macOS they also hide
	// the Dock icon (a packaged bundle says so from the start with
	// LSUIElement in mygo.config.ts).
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	// Another double-click opens the browser in the running instance.
	mygo.App.OnSecondInstance(func(_ []string, _ string) {
		openBrowser(url)
	})

	mygo.App.WhenReady(func() {
		// The saved language comes from the web UI; the first launch has
		// none and follows the system.
		if app.Settings.App.Language == "" {
			if languages := mygo.App.PreferredLanguages(); len(languages) > 0 {
				i18n.SetLanguage(languages[0])
			}
		}

		tray, err := mygo.NewTray(mygo.TrayOptions{
			ID:             appName,
			Icon:           trayIcon(),
			IconIsTemplate: runtime.GOOS == "darwin",
			ToolTip:        appName,
			Menu:           trayMenu(url, logPath),
		})
		if err != nil {
			fatal(fmt.Errorf("no menu bar icon: %w", err))
		}
		// The web UI saves the language it shows; relabel the menu as it
		// changes.
		i18n.OnLanguageChanged(func(string) {
			tray.SetMenu(trayMenu(url, logPath))
		})
		if runtime.GOOS == "windows" {
			// The notification area opens TrayOptions.Menu on right-click;
			// the taskbar icon's primary click opens it too, as people
			// click a taskbar icon.
			tray.OnClick(func() { tray.PopUpMenu() })
		}
		mygo.App.OnWillQuit(func(*mygo.QuitEvent) { tray.Destroy() })

		// The service is up: the launch lands in the browser.
		openBrowser(url)
	})
	return mygo.App.Run()
}

// trayMenu builds the menu bar menu in the language the app shows.
func trayMenu(url, logPath string) *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: i18n.Localize("tray.open_browser"), Click: func(*mygo.MenuItem, *mygo.Window) { openBrowser(url) }},
		{Label: i18n.Localize("tray.open_log"), Click: func(*mygo.MenuItem, *mygo.Window) { openLogFile(logPath) }},
		mygo.Separator(),
		{Role: mygo.RoleQuit, Label: i18n.Localize("tray.quit")},
	})
}

// openBrowser opens url with the default browser.
func openBrowser(url string) {
	if err := mygo.Shell.OpenExternal(url); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

// openLogFile opens the service log in the default editor; when no app
// opens it (or the file does not exist yet), it shows the folder holding it.
func openLogFile(path string) {
	if err := mygo.Shell.OpenPath(path); err != nil {
		if err := mygo.Shell.OpenPath(filepath.Dir(path)); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

// startService starts the atvloadly service on a random loopback port and
// returns its URL. stop releases the port and stops the background jobs.
func startService() (url string, stop func(), err error) {
	addr, stop, err := bootstrap.Run(bootstrap.Options{Port: -1})
	if err != nil {
		return "", nil, err
	}
	return "http://" + addr + "/", stop, nil
}

// trayIcon returns the mark the menu bar (macOS) or notification area
// (Windows) shows.
func trayIcon() []byte {
	if runtime.GOOS == "darwin" {
		return trayDarwinIcon
	}
	return trayWindowsIcon
}

// dataDir returns the desktop data directory: the app's user data directory
// on each OS, before the app runs.
func dataDir() string {
	if base := os.Getenv("ATVLOADLY_DATA_DIR"); base != "" {
		return filepath.Join(base, appName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return appName
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", appName)
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", appName)
	default:
		return filepath.Join(home, ".local", "share", appName)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

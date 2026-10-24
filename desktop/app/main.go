// The atvloadly desktop app: a mygo webview shell around the atvloadly
// server. The shell starts the service on a loopback port of its own and
// shows the existing web frontend in a window of the OS webview.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/bootstrap"
	"github.com/egoist/mygo"
)

const appName = "atvloadly"

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
	// directory; surface the running window instead of failing on the
	// database or port later.
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	mygo.App.OnSecondInstance(func(_ []string, _ string) {
		mygo.App.Focus()
	})

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

	mygo.App.SetName(appName)
	mygo.App.SetVersion(app.Version.Version)
	mygo.App.OnWindowAllClosed(func() {
		stop()
		mygo.App.Quit()
	})
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title: appName,
			URL:   url,
			// 1280 is the frontend's xl breakpoint: below it the pages fall
			// back to the stacked tablet layout, so the default window opens
			// above it.
			Width:           1440,
			Height:          900,
			MinWidth:        960,
			MinHeight:       640,
			BackgroundColor: "light-dark(#f5f5f7, #1e1e1e)",
			StateKey:        "main",
		})
	})
	return mygo.App.Run()
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

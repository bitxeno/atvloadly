package release

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/i18n"
)

func TestNewInfoAvailable(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v0.8.1", "v0.8.2", true},
		{"0.8.1", "v0.8.2", true},
		{"v0.8.2-beta.1", "v0.8.2", true},
		{"v0.8.2", "v0.8.2", false},
		{"v0.9.0", "v0.8.2", false},
		{"unknown", "v0.8.2", false},
		{"master", "v0.8.2", false},
		{"v0.8.1", "", false},
	}
	for _, c := range cases {
		got := newInfo(c.current, githubRelease{TagName: c.latest}).Available
		if got != c.want {
			t.Errorf("newInfo(%q, %q).Available = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestCheckNotifiesOncePerRelease(t *testing.T) {
	i18n.Init(os.DirFS("../.."))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/"+Repo+"/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.9.0","html_url":"https://github.com/bitxeno/atvloadly/releases/tag/v0.9.0"}`))
	}))
	defer server.Close()

	oldBase, oldSend, oldSettings, oldVersion := apiBaseURL, sendNotification, app.Settings, app.Version
	defer func() {
		apiBaseURL, sendNotification, app.Settings, app.Version = oldBase, oldSend, oldSettings, oldVersion
	}()
	apiBaseURL = server.URL
	app.Settings = &app.SettingsConfiguration{}
	app.Settings.Notification.Enabled = true
	app.Settings.Update.ReleaseNotify = true
	app.Version.Version = "v0.8.2"

	var sent, checks, handled []string
	sendNotification = func(title, message string) error {
		sent = append(sent, title)
		return nil
	}
	oldChecked, oldAnnounced := checked, announced
	defer func() { checked, announced = oldChecked, oldAnnounced }()
	OnCheck(func(info Info) { checks = append(checks, info.Latest) })
	OnNewRelease(func(info Info) { handled = append(handled, info.Latest) })

	for i := 0; i < 2; i++ {
		if err := Check(); err != nil {
			t.Fatalf("Check() error = %v", err)
		}
	}
	if len(sent) != 1 || len(handled) != 1 {
		t.Fatalf("sent %d notifications and %d new-release calls, want 1 each", len(sent), len(handled))
	}
	if len(checks) != 2 {
		t.Fatalf("got %d check calls, want 2", len(checks))
	}
	if got := app.Settings.Update.NotifiedRelease; got != "v0.9.0" {
		t.Fatalf("NotifiedRelease = %q, want v0.9.0", got)
	}

	info := Latest()
	if !info.Available || info.Latest != "v0.9.0" || info.Current != "v0.8.2" {
		t.Fatalf("Latest() = %+v", info)
	}
}

func TestAnnounceSkippedWhenDisabled(t *testing.T) {
	oldSend, oldSettings := sendNotification, app.Settings
	defer func() { sendNotification, app.Settings = oldSend, oldSettings }()
	app.Settings = &app.SettingsConfiguration{}
	app.Settings.Notification.Enabled = true
	app.Settings.Update.ReleaseNotify = false

	calls := 0
	sendNotification = func(string, string) error { calls++; return nil }
	announce(Info{Latest: "v0.9.0", Available: true}, []func(Info){func(Info) { calls++ }})

	if calls != 0 {
		t.Fatalf("got %d notifications while release notifications are disabled", calls)
	}
	if app.Settings.Update.NotifiedRelease != "" {
		t.Fatalf("NotifiedRelease = %q, want it unset so enabling later still notifies", app.Settings.Update.NotifiedRelease)
	}
}

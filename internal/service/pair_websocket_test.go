package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	ws "github.com/fasthttp/websocket"
	"github.com/gofiber/contrib/websocket"
)

// Exercise the real subprocess and WebSocket transport without an Apple TV.
func TestPairWebsocketReportsProcessResultWithoutSuccessLogs(t *testing.T) {
	for _, tt := range []struct{ name, script, terminal string }{
		{"success", "exit 0", "Pairing Succeeded!"},
		{"failure", "printf 'Invalid PIN' >&2\nexit 3", "Pairing Failed!"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := setTestDataDir(t)
			previous := app.Settings
			app.Settings = &app.SettingsConfiguration{}
			t.Cleanup(func() { app.Settings = previous })
			engineDir := t.TempDir()
			script := "#!/bin/sh\nprintf 'Enter PIN:' >&2\nread -r pin\n" + tt.script + "\n"
			if err := os.WriteFile(filepath.Join(engineDir, "plumesign"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", engineDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("XDG_CONFIG_HOME", dataDir)
			done := make(chan struct{})
			upgrader := ws.Upgrader{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(done)
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer func() { _ = conn.Close() }()
				mgr := manager.NewWebsocketManager(&websocket.Conn{Conn: conn})
				pair := manager.NewPairManager()
				pair.OnOutput(mgr.WriteMessage)
				workerDone := make(chan struct{})
				go func() {
					defer close(workerDone)
					runPairMessage(mgr, pair, model.Device{Name: "Test TV", IP: "192.0.2.1", Port: 1234})
				}()
				defer func() { mgr.Cancel(); <-workerDone; pair.Close() }()
				for {
					msg, err := mgr.ReadMessage()
					if err != nil {
						return
					}
					pair.Write([]byte(msg.Data + "\n"))
				}
			}))
			defer server.Close()
			conn, _, err := ws.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = conn.Close()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("pair handler did not exit")
				}
			}()
			if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			_, prompt, err := conn.ReadMessage()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(prompt), "Enter PIN") {
				t.Fatalf("missing prompt: %q", prompt)
			}
			if err := conn.WriteJSON(model.Message{Type: model.MessageTypePairConfirm, Data: "123456"}); err != nil {
				t.Fatal(err)
			}
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(string(message), "Pairing ") {
					if string(message) != tt.terminal {
						t.Fatalf("terminal message = %q, want %q", message, tt.terminal)
					}
					break
				}
			}
		})
	}
}

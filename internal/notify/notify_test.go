package notify

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
)

func TestSendWithConfig_WebhookJSONBodyTemplate(t *testing.T) {
	var gotBody string
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		gotUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var settings app.SettingsConfiguration
	settings.Notification.Enabled = true
	settings.Notification.Type = "webhook"
	settings.Notification.Webhook.URL = server.URL
	settings.Notification.Webhook.Method = http.MethodPost
	settings.Notification.Webhook.ContentType = "application/json"
	settings.Notification.Webhook.Body = `{"content":"Installation Failed: {{title}} - {{message}}"}`

	if err := SendWithConfig("atvloadly", "test message", settings); err != nil {
		t.Fatalf("SendWithConfig() error = %v", err)
	}

	want := `{"content":"Installation Failed: atvloadly - test message"}`
	if gotBody != want {
		t.Fatalf("unexpected webhook body\nwant: %s\ngot:  %s", want, gotBody)
	}
	if gotUserAgent != "atvloadly" {
		t.Fatalf("unexpected User-Agent\nwant: %s\ngot:  %s", "atvloadly", gotUserAgent)
	}
}

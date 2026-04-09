package notify

import (
	"encoding/json"
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

func TestSendWithConfig_WebhookJSONBodyTemplateEscapesSpecialCharacters(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("request body should be valid JSON: %v\nbody: %s", err, string(body))
		}
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

	title := `release "beta"`
	message := "line1\\line2\nline3"
	if err := SendWithConfig(title, message, settings); err != nil {
		t.Fatalf("SendWithConfig() error = %v", err)
	}

	want := "Installation Failed: release \"beta\" - line1\\line2\nline3"
	if got["content"] != want {
		t.Fatalf("unexpected content\nwant: %q\ngot:  %q", want, got["content"])
	}
}

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

func TestIsJSONContentType(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{
			name:        "plain json",
			contentType: "application/json",
			expected:    true,
		},
		{
			name:        "json with charset",
			contentType: "application/json; charset=utf-8",
			expected:    true,
		},
		{
			name:        "non json",
			contentType: "text/plain",
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isJSONContentType(tc.contentType)
			if actual != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestSanitizeJSONTemplateValue(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escape quote and newline",
			input:    "hello \"world\"\nline2",
			expected: "hello \\\"world\\\"\\nline2",
		},
		{
			name:     "remove invalid utf8 bytes",
			input:    string([]byte{'o', 'k', 0xff, 'x'}),
			expected: "okx",
		},
		{
			name:     "escape tab and carriage return",
			input:    "a\tb\rc",
			expected: "a\\tb\\rc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := sanitizeJSONTemplateValue(tc.input)
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

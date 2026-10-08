package exec

import (
	"strings"
	"testing"
	"time"
)

// A secret argument is masked in the debug log while the process still gets
// the real value. -p carries a password for some subcommands and a package
// path for others, so masking is by value, not by position.
func TestCommand_LogLineMasksSecrets(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		secrets []string
		want    string
	}{
		{
			name:    "password is masked",
			args:    []string{"certificate", "import", "-u", "user@example.com", "-p", "s3cret", "-i", "/tmp/a.p12"},
			secrets: []string{"s3cret"},
			want:    "plumesign certificate import -u user@example.com -p *** -i /tmp/a.p12",
		},
		{
			name:    "a path that is not a secret is kept",
			args:    []string{"sign", "-u", "user@example.com", "-p", "app.ipa"},
			secrets: []string{"other-secret"},
			want:    "plumesign sign -u user@example.com -p app.ipa",
		},
		{
			name:    "an empty secret matches nothing",
			args:    []string{"certificate", "import", "-p", "", "-i", "/tmp/a.p12"},
			secrets: []string{""},
			want:    "plumesign certificate import -p  -i /tmp/a.p12",
		},
		{
			name:    "every occurrence is masked",
			args:    []string{"login", "-u", "user", "-p", "twice", "--again", "twice"},
			secrets: []string{"twice"},
			want:    "plumesign login -u user -p *** --again ***",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCommand("plumesign", tc.args...).WithSecret(tc.secrets...)
			if got := cmd.logLine(); got != tc.want {
				t.Fatalf("logLine() = %q, want %q", got, tc.want)
			}
			// The real argument list is untouched.
			if strings.Join(cmd.Args, "\x00") != strings.Join(tc.args, "\x00") {
				t.Fatalf("Args changed: %q", cmd.Args)
			}
		})
	}
}

func TestCommand_CombinedOutput(t *testing.T) {
	cmd := NewCommand("echo", "hello world")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if strings.TrimSpace(string(output)) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", strings.TrimSpace(string(output)))
	}

	// Test timeout
	cmd = NewCommand("sleep", "2").WithTimeout(1 * time.Second)
	_, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	}

	// Test error parsing
	// Simulate a command that outputs "error: something went wrong"
	cmd = NewCommand("sh", "-c", "echo 'Error: something went wrong' && exit 1")
	_, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "something went wrong") {
		t.Errorf("Expected error to contain 'something went wrong', got '%v'", err)
	}
}

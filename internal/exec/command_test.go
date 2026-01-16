package exec

import (
	"strings"
	"testing"
	"time"
)

func TestCommand_CombinedOutput(t *testing.T) {
	// Test success
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

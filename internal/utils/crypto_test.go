package utils

import "testing"

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{name: "standard email", email: "alice@example.com", want: "a***e@example.com"},
		{name: "short local part", email: "ab@example.com", want: "**@example.com"},
		{name: "no at sign", email: "abcde", want: "a***e"},
		{name: "empty", email: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskEmail(tt.email); got != tt.want {
				t.Fatalf("MaskEmail(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}

package service

import "testing"

func TestValidateCustomName(t *testing.T) {
	ok := []string{"", "Nemio", "My App", "App 2.0", "Media-Center", "影视", "串流 App"}
	bad := []string{"tv_shelf", " leading", "-dash", "under_score", "emoji 🎬", "a<b>"}
	for _, n := range ok {
		if err := ValidateCustomName(n); err != nil {
			t.Errorf("expected %q valid, got %v", n, err)
		}
	}
	for _, n := range bad {
		if err := ValidateCustomName(n); err == nil {
			t.Errorf("expected %q invalid", n)
		}
	}
	long := ""
	for i := 0; i < 65; i++ {
		long += "å"
	}
	if err := ValidateCustomName(long); err == nil {
		t.Error("expected 65-rune name invalid")
	}
}

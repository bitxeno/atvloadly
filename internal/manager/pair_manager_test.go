package manager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
)

func TestPairManagerDisconnectCancelsPINWait(t *testing.T) {
	setupAppleIDInstall(t, "pair-test")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plumesign"), []byte("#!/bin/sh\nprintf 'Enter PIN:'\nread -r pin\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	mgr := NewPairManager()
	defer mgr.Close()
	ready := make(chan struct{})
	var once sync.Once
	mgr.OnOutput(func(s string) {
		if strings.Contains(s, "Enter PIN") {
			once.Do(func() { close(ready) })
		}
	})
	done := make(chan error, 1)
	go func() { done <- mgr.Start(context.Background(), model.Device{IP: "192.0.2.1", Port: 1234}) }()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("missing PIN prompt")
	}
	if err := mgr.Start(context.Background(), model.Device{}); err == nil {
		t.Fatal("concurrent pairing was accepted")
	}
	mgr.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled pairing reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("disconnect left pairing running")
	}
	mgr.Write([]byte("123456\n"))
	if err := mgr.Start(context.Background(), model.Device{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("closed manager Start: %v", err)
	}
}

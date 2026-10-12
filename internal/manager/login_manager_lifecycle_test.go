package manager

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestLoginWriteWithoutActiveProcess(t *testing.T) {
	mgr := &LoginManager{}
	mgr.Write([]byte("123456\n"))
}

func TestLoginWriteToActivePipe(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()

	mgr := &LoginManager{stdin: writer}
	mgr.Write([]byte("123456\n"))
	buf := make([]byte, len("123456\n"))
	if _, err := io.ReadFull(reader, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "123456\n" {
		t.Fatalf("wrong input: %q", buf)
	}
}

func TestLoginWriteConcurrentShutdown(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	mgr := &LoginManager{stdin: writer}

	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				mgr.Write([]byte("x"))
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		mgr.stateMu.Lock()
		mgr.stdin = nil
		mgr.stateMu.Unlock()
		_ = writer.Close()
	}()
	wg.Wait()
	mgr.Write([]byte("late 2FA"))
}

func TestLoginConcurrentStartIsRejected(t *testing.T) {
	mgr := &LoginManager{}
	mgr.runMu.Lock()
	defer mgr.runMu.Unlock()
	err := mgr.Start(context.Background(), "user", "password")
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("expected busy error, got %v", err)
	}
}

func TestLoginCloseCancelsActiveAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := &LoginManager{cancel: cancel}
	mgr.Close()
	if ctx.Err() != context.Canceled {
		t.Fatalf("Close did not cancel active attempt: %v", ctx.Err())
	}
	mgr.Close()
}

package manager

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
)

func TestPairWriteWithoutActiveProcess(t *testing.T) {
	mgr := &PairManager{}
	mgr.Write([]byte("1234\n"))
}

func TestPairWriteToActivePipe(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()

	mgr := &PairManager{stdin: writer}
	mgr.Write([]byte("1234\n"))
	buf := make([]byte, len("1234\n"))
	if _, err := io.ReadFull(reader, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "1234\n" {
		t.Fatalf("wrong pairing input: %q", buf)
	}
}

func TestPairWriteConcurrentShutdown(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	mgr := &PairManager{stdin: writer}

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
	mgr.Write([]byte("late PIN"))
}

func TestPairConcurrentStartIsRejected(t *testing.T) {
	mgr := &PairManager{}
	mgr.runMu.Lock()
	defer mgr.runMu.Unlock()
	err := mgr.Start(context.Background(), model.Device{})
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("expected pairing busy error, got %v", err)
	}
}

func TestPairCloseCancelsActiveAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := &PairManager{cancel: cancel}
	mgr.Close()
	if ctx.Err() != context.Canceled {
		t.Fatalf("Close did not cancel active pairing: %v", ctx.Err())
	}
	mgr.Close()
}

func TestPairConcurrentClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := &PairManager{cancel: cancel}

	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.Close()
		}()
	}
	wg.Wait()
	if ctx.Err() != context.Canceled {
		t.Fatalf("Close did not cancel active pairing: %v", ctx.Err())
	}
}

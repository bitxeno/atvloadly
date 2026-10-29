package diagnostics

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestCPUProfilerLoopbackAndShutdown(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if host, _, err := net.SplitHostPort(listener.Addr().String()); err != nil || host != "127.0.0.1" {
		t.Fatalf("profiler exposed beyond loopback: %s (%v)", listener.Addr(), err)
	}
	stop := serveCPUProfile(listener)
	defer stop()
	url := "http://" + listener.Addr().String()
	resp, err := http.Get(url + "/debug/pprof/goroutine?debug=1")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(data), "goroutine") {
		t.Fatalf("goroutine profiling failed: status=%d err=%v", resp.StatusCode, err)
	}
	resp, err = http.Get(url + "/not-a-profile")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected route status: %d", resp.StatusCode)
	}
	stop()
	stop() // idempotent, no hang
	_, err = http.Get(url + "/debug/pprof/")
	if err == nil {
		t.Fatal("profiling listener still reachable after stop")
	}
}

func TestCPUProfileCapture(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stop := serveCPUProfile(listener)
	defer stop()
	resp, err := http.Get("http://" + listener.Addr().String() + "/debug/pprof/profile?seconds=1")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK || len(data) == 0 {
		t.Fatalf("cpu profile failed: bytes=%d status=%d err=%v", len(data), resp.StatusCode, err)
	}
}

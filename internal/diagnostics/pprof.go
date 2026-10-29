package diagnostics

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"
)

// StartCPUProfileServer provides opt-in profiling only on the container's
// loopback interface. It is never connected to the public application router.
func StartCPUProfileServer() (func(), error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:6060")
	if err != nil {
		return nil, fmt.Errorf("listen on local profiling port: %w", err)
	}
	return serveCPUProfile(listener), nil
}

func serveCPUProfile(listener net.Listener) func() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	for _, name := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
		mux.Handle("GET /debug/pprof/"+name, pprof.Handler(name))
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Serve(listener)
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = server.Close()
			<-done
		})
	}
}

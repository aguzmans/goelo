// credserve serves goelo's stateless rating core over HTTP (POST /rate). It binds
// to localhost by default — the rating engine is a trusted internal dependency, not
// a public endpoint — so put it behind your own auth/ingress before exposing it.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aguzmans/goelo/service"
)

// version is set by GoReleaser for tagged builds.
var version = "dev"

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "host:port to listen on (localhost-only by default)")
	flag.Parse()

	srv := newServer(*addr)
	if err := serve(srv); err != nil {
		fmt.Fprintln(os.Stderr, "credserve:", err)
		os.Exit(1)
	}
}

// newServer builds the HTTP server with sane timeouts. Split out so tests can
// assert the wiring without binding a port.
func newServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           service.NewHandler(version),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// serve runs the server until SIGINT/SIGTERM, then shuts down gracefully.
func serve(srv *http.Server) error {
	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stderr, "credserve %s listening on %s\n", version, srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-stop:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

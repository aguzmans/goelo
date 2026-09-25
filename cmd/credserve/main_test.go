package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewServerWiring(t *testing.T) {
	srv := newServer("127.0.0.1:0")
	if srv.Addr != "127.0.0.1:0" {
		t.Errorf("addr = %q, want 127.0.0.1:0", srv.Addr)
	}
	if srv.Handler == nil {
		t.Fatal("handler not wired")
	}
	if srv.ReadHeaderTimeout == 0 {
		t.Error("ReadHeaderTimeout should be set")
	}

	// The wired handler serves the health endpoint.
	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d, want 200", rr.Code)
	}
}

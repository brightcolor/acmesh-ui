package certs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestCheckEndpoint(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	served := CheckEndpoint(context.Background(), u.Hostname(), port)
	if !served.Reachable {
		t.Fatalf("expected endpoint reachable: %s", served.Error)
	}
	if served.Fingerprint == "" {
		t.Fatalf("expected a fingerprint")
	}
	// The served fingerprint must equal the server's own certificate fingerprint.
	want := Fingerprint(srv.Certificate())
	if served.Fingerprint != want {
		t.Fatalf("fingerprint mismatch: got %s want %s", served.Fingerprint, want)
	}
}

func TestCheckEndpointUnreachable(t *testing.T) {
	// Port 1 is not listening; expect a clean unreachable result, not a panic.
	served := CheckEndpoint(context.Background(), "127.0.0.1", 1)
	if served.Reachable {
		t.Fatalf("expected unreachable")
	}
	if served.Error == "" {
		t.Fatalf("expected an error message")
	}
}

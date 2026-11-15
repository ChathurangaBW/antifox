package traffic

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mproxy "github.com/lqqyt2423/go-mitmproxy/proxy"
)

// Regression: the inherited BaseAddon method did not write a response, leaving
// go-mitmproxy's entry handler to emit its Chinese fallback. Exercise our addon
// over loopback HTTP without a database, task engine, or forwarding destination.
// CONNECT/forwarding are not exercised: this callback only handles direct access.
func TestDirectProxyMessageEnglish(t *testing.T) {
	addon := &sink{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addon.AccessProxyServer(r, w)
	}))
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = 5 * time.Second

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{"root", http.MethodGet, "/"},
		{"nested_path", http.MethodGet, "/not-an-app-page?message=do-not-reflect-this"},
		{"post", http.MethodPost, "/"},
		{"head", http.MethodHead, "/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, server.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
			if got := resp.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
				t.Errorf("Content-Type = %q, want plain UTF-8 text", got)
			}
			if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
			want := "This is a proxy server; direct requests are not supported.\n"
			if tc.method == http.MethodHead {
				want = ""
			}
			if string(body) != want {
				t.Errorf("body = %q, want %q", body, want)
			}
			if strings.Contains(string(body), "do-not-reflect-this") {
				t.Error("request input was reflected in the response")
			}
		})
	}
}

// This reaches the dependency's real entry.ServeHTTP and its fallback check,
// rather than calling our callback directly. Only origin-form loopback requests
// are sent; no CONNECT, destination traffic, or task engine is started.
func TestDirectProxyEntryEnglish(t *testing.T) {
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := reservation.Addr().String()
	proxy, err := mproxy.NewProxy(&mproxy.Options{Addr: addr, CaRootPath: t.TempDir()})
	if err != nil {
		reservation.Close()
		t.Fatal(err)
	}
	proxy.AddAddon(&sink{})
	// The dependency exposes Start, but no Serve(listener). Release the selected
	// address immediately before Start; any bind failure is reported by finished.
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- proxy.Start() }()
	t.Cleanup(func() {
		_ = proxy.Close()
		select {
		case err := <-finished:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("proxy stopped: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("proxy did not stop")
		}
	})
	transport := &http.Transport{} // No environment proxy; loopback only.
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		resp, err := client.Get("http://" + addr + "/")
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := "This is a proxy server; direct requests are not supported.\n"
			if resp.StatusCode != http.StatusBadRequest || string(body) != want {
				t.Fatalf("entry response = HTTP %d %q; want HTTP 400 %q", resp.StatusCode, body, want)
			}
			if got := resp.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
				t.Fatalf("Content-Type = %q", got)
			}
			t.Logf("real proxy entry: HTTP %d; body=%q", resp.StatusCode, body)
			return
		}
		select {
		case startErr := <-finished:
			finished <- startErr // Preserve the result for cleanup.
			t.Fatalf("proxy startup: %v", startErr)
		case <-deadline.C:
			t.Fatalf("loopback proxy did not become ready: %v", err)
		case <-tick.C:
		}
	}
}

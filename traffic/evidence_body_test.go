package traffic

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeResponse stores a response.http for an exchange id the way record does.
func writeResponse(t *testing.T, tr *Traffic, id, head, body string) {
	t.Helper()
	insertExchange(t, tr, id, "GET", "https://h/"+id, "", "")
	if err := os.MkdirAll(filepath.Join(tr.dir, id), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tr.dir, id, "response.http"), []byte(head+"\n\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResponseBody(t *testing.T) {
	tr := newTestTraffic(t)

	writeResponse(t, tr, "plain", "HTTP/1.1 200 OK\nContent-Type: text/plain", "hello evidence")
	if b, cut, err := tr.ResponseBody("plain", 1<<20); err != nil || cut || string(b) != "hello evidence" {
		t.Fatalf("plain: %q cut=%v err=%v", b, cut, err)
	}

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte(`{"secret":"decoded-ok"}`))
	_ = zw.Close()
	writeResponse(t, tr, "gzipped", "HTTP/1.1 200 OK\nContent-Encoding: gzip", gz.String())
	if b, _, err := tr.ResponseBody("gzipped", 1<<20); err != nil || !strings.Contains(string(b), "decoded-ok") {
		t.Fatalf("gzip: %q err=%v", b, err)
	}

	writeResponse(t, tr, "corrupt", "HTTP/1.1 200 OK\nContent-Encoding: gzip", "not gzip at all")
	if _, _, err := tr.ResponseBody("corrupt", 1<<20); err == nil {
		t.Fatal("an undecodable body must be an error, never raw bytes")
	}

	big := strings.Repeat("A", maxInlineBody+1024) + "PROOF-LINE"
	ptr := tr.bodyOrBlob([]byte(big))
	writeResponse(t, tr, "blob", "HTTP/1.1 200 OK", ptr)
	if !strings.HasPrefix(ptr, "@blob ") {
		t.Skipf("body under the inline limit (%d bytes); blob path not exercised", len(big))
	}
	if b, _, err := tr.ResponseBody("blob", 1<<20); err != nil || !strings.Contains(string(b), "PROOF-LINE") {
		t.Fatalf("blob: %q err=%v", b, err)
	}

	writeResponse(t, tr, "long", "HTTP/1.1 200 OK", strings.Repeat("x", 50))
	if b, cut, err := tr.ResponseBody("long", 10); err != nil || !cut || len(b) != 10 {
		t.Fatalf("cap: len=%d cut=%v err=%v", len(b), cut, err)
	}
}

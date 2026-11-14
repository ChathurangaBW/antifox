package traffic

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	mproxy "github.com/lqqyt2423/go-mitmproxy/proxy"
)

// reBlobPointer matches the pointer bodyOrBlob writes in place of a large body.
var reBlobPointer = regexp.MustCompile(`^@blob sha256:([0-9a-f]{64}) `)

// ResponseBody returns the response body of one recorded exchange for evidence
// checks: large bodies are read from their blob file, the declared
// Content-Encoding is decoded, and at most limit bytes are returned (truncated
// reports that the body was longer).
//
// Unlike Get, which favours display and returns undecodable bytes unchanged, this
// fails closed: an encoding that cannot be decoded, including one cut short by
// the read cap, is an error. A verification check must never compare a quote
// against compressed bytes and report that as a negative result.
func (t *Traffic) ResponseBody(id string, limit int) (body []byte, truncated bool, err error) {
	var rel string
	if err = t.db.QueryRow(`SELECT path FROM exchanges WHERE id=?`, id).Scan(&rel); err != nil {
		return nil, false, err
	}
	raw, err := os.ReadFile(filepath.Join(t.dir, rel, "response.http"))
	if err != nil {
		return nil, false, err
	}
	s := string(raw)
	i := strings.Index(s, "\n\n")
	if i < 0 {
		return nil, false, nil // no body recorded
	}
	head, inline := s[:i], s[i+2:]

	var data []byte
	if m := reBlobPointer.FindStringSubmatch(inline); m != nil {
		h := m[1]
		f, err := os.Open(filepath.Join(t.dir, "_blobs", "sha256", h[:2], h[2:4], h+".bin"))
		if err != nil {
			return nil, false, fmt.Errorf("blob body: %w", err)
		}
		defer f.Close()
		data, err = io.ReadAll(io.LimitReader(f, int64(limit)+1))
		if err != nil {
			return nil, false, err
		}
	} else {
		data = []byte(inline)
	}
	if len(data) > limit {
		data, truncated = data[:limit], true
	}

	enc := ""
	for _, line := range strings.Split(head, "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "Content-Encoding") {
			enc = strings.TrimSpace(v)
			break
		}
	}
	if enc == "" || strings.EqualFold(enc, "identity") || len(data) == 0 {
		return data, truncated, nil
	}
	if truncated {
		return nil, true, fmt.Errorf("%s body larger than the read cap cannot be decoded", enc)
	}
	h := http.Header{}
	h.Set("Content-Encoding", enc)
	dec, err := (&mproxy.Response{Header: h, Body: data}).DecodedBody()
	if err != nil || dec == nil {
		return nil, false, fmt.Errorf("decode %s body: %v", enc, err)
	}
	if len(dec) > limit {
		return dec[:limit], true, nil
	}
	return dec, false, nil
}

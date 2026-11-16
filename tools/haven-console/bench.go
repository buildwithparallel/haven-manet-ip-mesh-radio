package main

// Bench: the Field tab's speed test and internet pings, answered by the gate.
// The speed test measures the phone-to-gate path across every HaLow hop. It
// stops at the gate, so it never includes the internet line.

import (
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
)

const maxTestMB = 16

var zeros = make([]byte, 32<<10)

// speedDown sends ?mb= megabytes of zeros for the phone to time.
func speedDown(w http.ResponseWriter, r *http.Request) {
	mb, _ := strconv.Atoi(r.URL.Query().Get("mb"))
	if mb < 1 {
		mb = 1
	}
	if mb > maxTestMB {
		mb = maxTestMB
	}
	total := mb << 20
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(total))
	for sent := 0; sent < total; sent += len(zeros) {
		n := min(len(zeros), total-sent)
		if _, err := w.Write(zeros[:n]); err != nil {
			return
		}
	}
}

// speedUp reads and discards what the phone uploads.
func speedUp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	n, _ := io.Copy(io.Discard, io.LimitReader(r.Body, maxTestMB<<20))
	writeJSON(w, map[string]any{"bytes": n})
}

var resolvers = []struct{ name, ip string }{
	{"Cloudflare", "1.1.1.1"},
	{"Google", "8.8.8.8"},
	{"Quad9", "9.9.9.9"},
}

var pingTime = regexp.MustCompile(`time=([0-9.]+) ?ms`)

// pingHandler pings one public resolver from the gate. ?n= picks which one.
func pingHandler(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	t := resolvers[((n%len(resolvers))+len(resolvers))%len(resolvers)]
	out, _ := exec.CommandContext(r.Context(), "ping", "-c", "1", "-W", "1", t.ip).CombinedOutput()
	res := map[string]any{"name": t.name, "ok": false}
	if m := pingTime.FindSubmatch(out); m != nil {
		if ms, err := strconv.ParseFloat(string(m[1]), 64); err == nil {
			res["ok"], res["ms"] = true, ms
		}
	}
	writeJSON(w, res)
}

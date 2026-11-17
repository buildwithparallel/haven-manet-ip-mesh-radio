package main

// Finding add-on services on the mesh. Some tabs use a service that runs on
// one node, not the gate: the SDR agent (port 8090) and ADS-B (dump1090's
// SkyAware, port 8080) run on the node with the RTL-SDR. A serviceFinder
// checks every known node for one service, and passes that node's pages and
// data through the gate, so a phone on any Haven Wi-Fi can use it over this
// page's HTTPS.

import (
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type serviceFinder struct {
	col    *collector
	port   string                                   // the service's port on the node
	probe  string                                   // path that answers when the service runs
	prefix string                                   // this server's path for the pass-through, e.g. "/sdr-node/"
	fields func(body []byte) (map[string]any, bool) // what to report about a node; false: not the service

	mu    sync.Mutex
	nodes []map[string]any
	at    time.Time
}

// list checks every known node for the service, at most every 15 seconds.
// Each entry has "name" and "ip", plus whatever fields() returned.
func (f *serviceFinder) list() []map[string]any {
	f.mu.Lock()
	if time.Since(f.at) < 15*time.Second && f.nodes != nil {
		out := f.nodes
		f.mu.Unlock()
		return out
	}
	f.mu.Unlock()

	f.col.fresh(30 * time.Second) // one reading if the mesh has been idle; doesn't start polling
	v := f.col.view()
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		found = []map[string]any{}
	)
	seen := map[string]bool{}
	for _, n := range v.nodes {
		if n.ip == "" || n.err != "" || seen[n.ip] {
			continue
		}
		seen[n.ip] = true
		wg.Add(1)
		go func(name, ip string) {
			defer wg.Done()
			res, err := client.Get("http://" + ip + ":" + f.port + f.probe)
			if err != nil {
				return
			}
			defer res.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
			if res.StatusCode != http.StatusOK {
				return
			}
			info, ok := f.fields(body)
			if !ok {
				return
			}
			if info == nil {
				info = map[string]any{}
			}
			info["name"], info["ip"] = name, ip
			mu.Lock()
			found = append(found, info)
			mu.Unlock()
		}(v.name(n.mac), n.ip)
	}
	wg.Wait()
	sort.Slice(found, func(i, j int) bool { return found[i]["name"].(string) < found[j]["name"].(string) })

	f.mu.Lock()
	f.nodes, f.at = found, time.Now()
	f.mu.Unlock()
	return found
}

// handler answers {"nodes": [...]}; ?refresh=1 checks again now.
func (f *serviceFinder) handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("refresh") != "" {
		f.mu.Lock()
		f.at = time.Time{}
		f.mu.Unlock()
	}
	writeJSON(w, map[string]any{"nodes": f.list()})
}

// proxy serves <prefix><ip>/... from that node's service. Only nodes the
// finder has found are reachable this way.
func (f *serviceFinder) proxy(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, f.prefix)
	ip, path, _ := strings.Cut(rest, "/")
	ok := false
	for _, n := range f.list() {
		if n["ip"] == ip {
			ok = true
		}
	}
	if !ok {
		http.Error(w, "no such service at that address", http.StatusNotFound)
		return
	}
	target := &url.URL{Scheme: "http", Host: ip + ":" + f.port}
	p := httputil.NewSingleHostReverseProxy(target)
	p.FlushInterval = -1 // pass streams through as they arrive
	r.URL.Path = "/" + path
	r.URL.RawPath = ""
	r.Host = target.Host
	p.ServeHTTP(w, r)
}

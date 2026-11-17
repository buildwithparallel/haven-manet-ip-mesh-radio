package main

// The Route tab: how traffic from one node reaches the gate, from the gate's
// own batman-adv table, read every second. A reroute is logged with how long
// it took after the node it used went silent.

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const silentAfter = 3 * time.Second // not heard for this long = silent

type origEntry struct {
	Orig     string   `json:"orig_address"`
	Neigh    string   `json:"neigh_address"`
	Best     bool     `json:"best"`
	LastSeen int      `json:"last_seen_msecs"`
	TP       *float64 `json:"throughput"` // kbit/s
}

type routeEvent struct {
	T    int64    `json:"t"` // unix ms
	Kind string   `json:"kind"`
	Node string   `json:"node"`
	Path []string `json:"path,omitempty"` // node to gate, by name
	By   string   `json:"by,omitempty"`   // the silent node that forced a reroute
	Secs *float64 `json:"secs,omitempty"` // seconds from going silent to rerouted
}

type router struct {
	col     *collector
	gateMAC string

	mu          sync.Mutex
	origs       map[string][]origEntry
	last        map[string]time.Duration // shortest last-seen per originator
	paths       map[string][]string      // gate first, by MAC
	seen        map[string]bool
	silentSince map[string]time.Time
	events      []routeEvent
	tickedAt    time.Time
}

func newRouter(col *collector) *router {
	return &router{col: col, gateMAC: meshMAC(), origs: map[string][]origEntry{}, last: map[string]time.Duration{},
		paths: map[string][]string{}, seen: map[string]bool{}, silentSince: map[string]time.Time{}}
}

// meshMAC is the gate's own HaLow MAC: the first interface batman-adv uses.
func meshMAC() string {
	out, _ := exec.Command("batctl", "if").Output()
	iface := "wlh0"
	if m := regexp.MustCompile(`^([^:\s]+):`).FindSubmatch(out); m != nil {
		iface = string(m[1])
	}
	b, _ := os.ReadFile("/sys/class/net/" + iface + "/address")
	return strings.ToLower(strings.TrimSpace(string(b)))
}

var traceHop = regexp.MustCompile(`(?m)^\s*\d+:\s+([0-9a-f:]{17})`)
var traceLost = regexp.MustCompile(`(?m)^\s*\d+:\s+\*`)

// trace returns the real hop-by-hop path from the gate, or nil.
func (r *router) trace(dest string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "batctl", "tr", "-n", dest).Output()
	if err != nil || traceLost.Match(out) {
		return nil
	}
	path := []string{r.gateMAC}
	for _, m := range traceHop.FindAllSubmatch(out, -1) {
		path = append(path, string(m[1]))
	}
	if len(path) < 2 || path[len(path)-1] != dest {
		return nil
	}
	return path
}

// run checks routes every second while someone is looking (see
// activity.go), and does nothing otherwise. After an idle spell it starts
// over, so a change that happened while nobody watched isn't logged as a
// reroute with a made-up time.
func (r *router) run() {
	for {
		if watchers.wait() {
			r.mu.Lock()
			r.origs, r.last = map[string][]origEntry{}, map[string]time.Duration{}
			r.paths, r.seen, r.silentSince = map[string][]string{}, map[string]bool{}, map[string]time.Time{}
			r.mu.Unlock()
		}
		r.tick()
		r.mu.Lock()
		r.tickedAt = time.Now()
		r.mu.Unlock()
		time.Sleep(time.Second)
	}
}

// waitTick waits (up to max) for a route check newer than one second, so the
// first answer after an idle spell has real routes in it.
func (r *router) waitTick(max time.Duration) {
	for end := time.Now().Add(max); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
		r.mu.Lock()
		ok := time.Since(r.tickedAt) < time.Second
		r.mu.Unlock()
		if ok {
			return
		}
	}
}

func (r *router) tick() {
	out, err := exec.Command("batctl", "oj").Output()
	var list []origEntry
	if err == nil {
		_ = json.Unmarshal(out, &list)
	}
	now := time.Now()
	origs := map[string][]origEntry{}
	last := map[string]time.Duration{}
	for _, o := range list {
		o.Orig, o.Neigh = strings.ToLower(o.Orig), strings.ToLower(o.Neigh)
		origs[o.Orig] = append(origs[o.Orig], o)
		d := time.Duration(o.LastSeen) * time.Millisecond
		if cur, ok := last[o.Orig]; !ok || d < cur {
			last[o.Orig] = d
		}
	}
	silent := func(mac string) bool {
		if mac == r.gateMAC {
			return false
		}
		d, ok := last[mac]
		return !ok || d > silentAfter
	}

	// Paths, worked out before taking the lock (trace can take a moment).
	paths := map[string][]string{}
	for orig, entries := range origs {
		for _, e := range entries {
			if !e.Best {
				continue
			}
			switch {
			case e.Neigh == orig:
				paths[orig] = []string{r.gateMAC, orig}
			case !silent(e.Neigh):
				if p := r.trace(orig); p != nil {
					paths[orig] = p
					break
				}
				fallthrough
			default:
				paths[orig] = []string{r.gateMAC, e.Neigh, orig}
			}
		}
	}

	v := r.col.view()
	r.mu.Lock()
	defer r.mu.Unlock()
	for mac := range origs {
		r.seen[mac] = true
	}
	for mac := range r.seen {
		s := silent(mac)
		if _, was := r.silentSince[mac]; s && !was {
			since := now.Add(-silentAfter)
			if d, ok := last[mac]; ok {
				since = now.Add(-d)
			}
			r.silentSince[mac] = since
			r.log(routeEvent{T: now.UnixMilli(), Kind: "silent", Node: v.name(mac)})
		} else if !s && was {
			delete(r.silentSince, mac)
			r.log(routeEvent{T: now.UnixMilli(), Kind: "back", Node: v.name(mac)})
		}
	}
	for mac := range r.seen {
		old, had := r.paths[mac]
		cur := paths[mac]
		if had && strings.Join(old, ">") == strings.Join(cur, ">") {
			continue
		}
		if !had && cur == nil {
			continue
		}
		ev := routeEvent{T: now.UnixMilli(), Kind: "route", Node: v.name(mac), Path: r.names(v, cur)}
		if had {
			ev.Kind = "reroute"
			for _, m := range old[1:] {
				if t, ok := r.silentSince[m]; ok && m != mac {
					secs := now.Sub(t).Seconds()
					ev.By, ev.Secs = v.name(m), &secs
					break
				}
			}
		}
		if cur == nil {
			ev.Kind = "noroute"
			delete(r.paths, mac)
		} else {
			r.paths[mac] = cur
		}
		r.log(ev)
	}
	r.origs, r.last = origs, last
}

func (r *router) log(ev routeEvent) {
	r.events = append(r.events, ev)
	if len(r.events) > 60 {
		r.events = r.events[len(r.events)-60:]
	}
}

// names turns a gate-first MAC path into names, node first and gate last.
func (r *router) names(v *view, path []string) []string {
	out := make([]string, 0, len(path))
	for i := len(path) - 1; i >= 0; i-- {
		out = append(out, v.name(path[i]))
	}
	return out
}

func (r *router) handler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Query().Get("live") != "" {
		watchers.touch()
		r.waitTick(3 * time.Second)
	}
	r.col.fresh(10 * time.Second)
	v := r.col.view()
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()

	type node struct {
		N      string `json:"n"`
		Gate   bool   `json:"gate,omitempty"`
		Silent bool   `json:"silent,omitempty"`
		Last   *int   `json:"last,omitempty"` // ms since heard
	}
	type alt struct {
		Via   string   `json:"via"` // "direct" or a node name
		TP    *float64 `json:"tp"`
		Best  bool     `json:"best,omitempty"`
		Stale bool     `json:"stale,omitempty"`
	}
	nodes := []node{{N: v.name(r.gateMAC), Gate: true}}
	macs := make([]string, 0, len(r.seen))
	for mac := range r.seen {
		if mac != r.gateMAC {
			macs = append(macs, mac)
		}
	}
	sort.Slice(macs, func(i, j int) bool { return v.name(macs[i]) < v.name(macs[j]) })
	dest := ""
	want := strings.ToLower(req.URL.Query().Get("to"))
	for _, mac := range macs {
		n := node{N: v.name(mac)}
		if d, ok := r.last[mac]; ok {
			ms := int(d.Milliseconds())
			n.Last = &ms
			n.Silent = d > silentAfter
		} else {
			n.Silent = true
		}
		nodes = append(nodes, n)
		if strings.ToLower(n.N) == want || mac == want {
			dest = mac
		}
	}
	out := map[string]any{"gate": v.name(r.gateMAC), "nodes": nodes, "now": now.UnixMilli()}
	if dest != "" {
		out["to"] = v.name(dest)
		path := r.paths[dest]
		status := "none"
		if path != nil {
			status = "direct"
			if len(path) > 2 {
				status = "relayed"
			}
			for _, m := range path[1:] {
				if t, ok := r.silentSince[m]; ok {
					status = "lost"
					out["lost"] = v.name(m)
					out["lostMs"] = now.Sub(t).Milliseconds()
					break
				}
			}
			out["path"] = r.names(v, path)
		}
		out["status"] = status
		alts := []alt{}
		for _, e := range r.origs[dest] {
			a := alt{Via: "direct", Best: e.Best, Stale: time.Duration(e.LastSeen)*time.Millisecond > silentAfter}
			if e.Neigh != dest {
				a.Via = v.name(e.Neigh)
				if _, s := r.silentSince[e.Neigh]; s {
					a.Stale = true
				}
			}
			if e.TP != nil {
				tp := float64(int(*e.TP/100+0.5)) / 10
				a.TP = &tp
				if e.Best {
					out["tp"] = tp
				}
			}
			alts = append(alts, a)
		}
		sort.Slice(alts, func(i, j int) bool { return alts[i].Best || (!alts[j].Best && alts[i].Via < alts[j].Via) })
		out["alts"] = alts
	}
	evs := []routeEvent{}
	for _, e := range r.events {
		if e.Kind == "silent" || e.Kind == "back" || (dest != "" && e.Node == v.name(dest)) {
			evs = append(evs, e)
		}
	}
	if len(evs) > 12 {
		evs = evs[len(evs)-12:]
	}
	out["events"] = evs
	writeJSON(w, out)
}

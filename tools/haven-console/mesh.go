package main

// Mesh: reads every node's agent over the mesh and turns the readings into
// the Field and Mesh tabs. Nodes are found from the gate's DHCP leases and
// neighbor table, so there is no node list to maintain.

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type peerReading struct {
	mac            string
	signal         *float64
	txMbps, rxMbps *float64
	plink          string
}

type routeReading struct {
	orig, next string
	tp         *float64
}

type apReading struct {
	ssid           string
	mhz, ch, width *float64
}

type nodeReading struct {
	host, ip, role, mac, dev string
	model                    string
	txp                      *float64
	gateway                  string
	peers                    []peerReading
	routes                   []routeReading
	stations                 map[string]*float64 // client MAC -> signal
	stationAP                map[string]int      // client MAC -> index into aps
	aps                      []apReading
	halowMHz, halowBW        *float64
	halowPBW, halowCh        *float64
	err                      string
	at                       time.Time
}

// agentJSON mirrors agent/haven-node.uc. Peers, routes and stations are arrays
// rather than objects to keep the answer small across HaLow.
type agentJSON struct {
	K      string   `json:"k"`
	Host   string   `json:"host"`
	Model  string   `json:"model"`
	IP     string   `json:"ip"`
	Role   string   `json:"role"`
	MAC    string   `json:"mac"`
	Dev    string   `json:"dev"`
	Txp    *float64 `json:"txp"`
	GW     *string  `json:"gw"`
	Peers  [][]any  `json:"peers"`
	Routes [][]any  `json:"routes"`
	Halow  []any    `json:"halow"`
	Aps    [][]any  `json:"aps"`
	Sta    [][]any  `json:"sta"`
	Error  string   `json:"error"`
}

func num(v any) *float64 {
	if f, ok := v.(float64); ok {
		return &f
	}
	return nil
}

func str(v any) string {
	s, _ := v.(string)
	return strings.ToLower(s)
}

func at(row []any, i int) any {
	if i < len(row) {
		return row[i]
	}
	return nil
}

func parseAgent(a agentJSON) *nodeReading {
	n := &nodeReading{host: a.Host, model: a.Model, ip: a.IP, role: a.Role, mac: strings.ToLower(a.MAC), dev: a.Dev, txp: a.Txp,
		stations: map[string]*float64{}, stationAP: map[string]int{}, at: time.Now()}
	n.halowMHz, n.halowBW = num(at(a.Halow, 0)), num(at(a.Halow, 1))
	n.halowPBW, n.halowCh = num(at(a.Halow, 2)), num(at(a.Halow, 3))
	for _, ap := range a.Aps {
		ssid, _ := at(ap, 0).(string) // keep the network name's case
		n.aps = append(n.aps, apReading{ssid: ssid, mhz: num(at(ap, 1)), ch: num(at(ap, 2)), width: num(at(ap, 3))})
	}
	if a.GW != nil {
		n.gateway = strings.ToLower(*a.GW)
	}
	for _, p := range a.Peers {
		n.peers = append(n.peers, peerReading{mac: str(at(p, 0)), signal: num(at(p, 1)), txMbps: num(at(p, 2)), rxMbps: num(at(p, 3)), plink: fmt.Sprint(at(p, 4))})
	}
	for _, r := range a.Routes {
		n.routes = append(n.routes, routeReading{orig: str(at(r, 0)), next: str(at(r, 1)), tp: num(at(r, 2))})
	}
	for _, s := range a.Sta {
		n.stations[str(at(s, 0))] = num(at(s, 1))
		if i := num(at(s, 2)); i != nil {
			n.stationAP[str(at(s, 0))] = int(*i)
		}
	}
	return n
}

type collector struct {
	localAgent string
	extra      []string
	client     *http.Client

	mu       sync.Mutex
	readings map[string]*nodeReading // by IP
	lastOK   map[string]time.Time
	notNode  map[string]time.Time
	selfIP   string

	refreshMu   sync.Mutex
	refreshedAt time.Time
}

func newCollector(localAgent string, extra []string) *collector {
	return &collector{
		localAgent: localAgent,
		extra:      extra,
		client: &http.Client{
			Timeout:   3 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
		readings: map[string]*nodeReading{},
		lastOK:   map[string]time.Time{},
		notNode:  map[string]time.Time{},
	}
}

func (c *collector) fetch(url string) (*nodeReading, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var a agentJSON
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&a); err != nil {
		return nil, err
	}
	if a.K != "node" {
		if a.Error != "" {
			return nil, fmt.Errorf("agent error: %s", a.Error)
		}
		return nil, fmt.Errorf("not a Haven node")
	}
	n := parseAgent(a)
	learnName(n.mac, n.host)
	return n, nil
}

// candidates lists addresses that might be nodes: the gate's DHCP leases and
// neighbor table on the mesh bridge, plus any configured extras.
func (c *collector) candidates() []string {
	seen := map[string]bool{}
	var out []string
	add := func(ip string) {
		if net.ParseIP(ip) == nil || seen[ip] || ip == c.selfIP {
			return
		}
		seen[ip] = true
		out = append(out, ip)
	}
	c.mu.Lock()
	for ip := range c.readings {
		add(ip)
	}
	c.mu.Unlock()
	for _, ip := range c.extra {
		add(ip)
	}
	for _, l := range readLeases() {
		add(l[1])
	}
	for ip := range readARP() {
		if strings.HasPrefix(ip, "10.41.") {
			add(ip)
		}
	}
	return out
}

func (c *collector) refresh() {
	local, err := c.fetch(c.localAgent)
	if err != nil {
		local = &nodeReading{host: hostname(), role: "gate", err: "local agent: " + err.Error(), at: time.Now()}
	}
	c.mu.Lock()
	c.selfIP = local.ip
	if local.ip != "" {
		c.readings[local.ip] = local
		c.lastOK[local.ip] = time.Now()
	}
	c.mu.Unlock()

	// One node at a time: in field testing, parallel reads over a slow HaLow
	// hop made the slowest node time out and vanish from the page.
	for _, ip := range c.candidates() {
		c.mu.Lock()
		skipUntil, skip := c.notNode[ip]
		_, known := c.readings[ip]
		c.mu.Unlock()
		if skip && time.Now().Before(skipUntil) {
			continue
		}
		r, err := c.fetch("http://" + ip + "/cgi-bin/haven-node")
		c.mu.Lock()
		switch {
		case err == nil:
			if r.ip == "" {
				r.ip = ip
			}
			c.readings[ip] = r
			c.lastOK[ip] = time.Now()
			delete(c.notNode, ip)
		case known:
			if time.Since(c.lastOK[ip]) > 30*time.Minute {
				delete(c.readings, ip) // gone for good
			} else {
				prev := c.readings[ip]
				c.readings[ip] = &nodeReading{host: prev.host, ip: ip, role: prev.role, mac: prev.mac, dev: prev.dev, model: prev.model, err: "no answer", at: time.Now(), stations: map[string]*float64{}}
			}
		default:
			c.notNode[ip] = time.Now().Add(10 * time.Minute) // a phone or laptop
		}
		c.mu.Unlock()
	}
}

// run reads every node every 3 seconds while someone is looking (see
// activity.go), and does nothing otherwise.
func (c *collector) run() {
	for {
		watchers.wait()
		start := time.Now()
		c.fresh(0)
		if d := 3*time.Second - time.Since(start); d > 0 {
			time.Sleep(d)
		}
	}
}

// fresh makes sure the readings are at most maxAge old, reading every node
// now if they aren't. Handlers use it so the first answer after an idle spell
// isn't out of date.
func (c *collector) fresh(maxAge time.Duration) {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	if maxAge > 0 && time.Since(c.refreshedAt) <= maxAge {
		return
	}
	c.refresh()
	c.refreshedAt = time.Now()
}

func (c *collector) snapshot() []*nodeReading {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*nodeReading, 0, len(c.readings))
	for _, r := range c.readings {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].role == "gate") != (out[j].role == "gate") {
			return out[i].role == "gate"
		}
		return out[i].host < out[j].host
	})
	return out
}

// view is one consistent reading of the mesh with lookups by HaLow MAC.
type view struct {
	nodes []*nodeReading
	byMAC map[string]*nodeReading
	gate  *nodeReading
	alias map[string]string
}

func (c *collector) view() *view {
	v := &view{nodes: c.snapshot(), byMAC: map[string]*nodeReading{}, alias: readBatHosts()}
	for _, n := range v.nodes {
		if n.mac != "" && n.err == "" {
			v.byMAC[n.mac] = n
		}
		if n.role == "gate" && n.err == "" && v.gate == nil {
			v.gate = n
		}
	}
	return v
}

func (v *view) name(mac string) string {
	if n := v.byMAC[mac]; n != nil && n.host != "" {
		return n.host
	}
	for _, n := range v.nodes {
		if n.mac == mac && n.host != "" {
			return n.host
		}
	}
	if a := v.alias[mac]; a != "" {
		return a
	}
	return mac
}

func routeTo(n *nodeReading, target string) *routeReading {
	for i := range n.routes {
		if n.routes[i].orig == target {
			return &n.routes[i]
		}
	}
	return nil
}

func peerOf(n *nodeReading, mac string) *peerReading {
	for i := range n.peers {
		if n.peers[i].mac == mac {
			return &n.peers[i]
		}
	}
	return nil
}

// chain walks the selected batman-adv next hop from start to target and
// returns the node names along the way. complete is false if a node on the
// way has no reading.
func (v *view) chain(start *nodeReading, target string) (names []string, complete bool) {
	names = []string{start.host}
	if start.mac == target {
		return names, true
	}
	cur := start
	seen := map[string]bool{}
	for hops := 0; hops < 8; hops++ {
		if seen[cur.mac] {
			return names, false
		}
		seen[cur.mac] = true
		r := routeTo(cur, target)
		if r == nil || r.next == "" {
			return names, false
		}
		names = append(names, v.name(r.next))
		if r.next == target {
			return names, true
		}
		next := v.byMAC[r.next]
		if next == nil {
			return names, false
		}
		cur = next
	}
	return names, false
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (c *collector) fieldHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("live") != "" { // a live view keeps the mesh polling; a one-off look doesn't
		watchers.touch()
	}
	c.fresh(10 * time.Second)
	v := c.view()
	ip := clientIP(r)
	mac := ""
	for _, l := range readLeases() {
		if l[1] == ip {
			mac = l[0]
		}
	}
	if mac == "" {
		mac = readARP()[ip]
	}
	out := map[string]any{"ip": ip}
	if v.gate != nil {
		out["gate"] = v.gate.host
	}
	if mac == "" {
		out["note"] = "This device was not found on the mesh."
		writeJSON(w, out)
		return
	}
	// The node that hears this phone loudest is the one it joined.
	var joined *nodeReading
	var sig *float64
	for _, n := range v.nodes {
		if s, ok := n.stations[mac]; ok && n.err == "" {
			if joined == nil || (s != nil && (sig == nil || *s > *sig)) {
				joined, sig = n, s
			}
		}
	}
	if joined == nil {
		out["note"] = "Not connected to a node's Wi-Fi."
		writeJSON(w, out)
		return
	}
	out["on"] = joined.host
	out["model"] = joined.model
	if sig != nil && *sig != 0 {
		out["wifi"] = *sig
	}
	// Every radio on this node, the phone's network first.
	mine, ok := joined.stationAP[mac]
	if !ok {
		mine = -1
	}
	aps := []map[string]any{}
	for i, ap := range joined.aps {
		a := map[string]any{"ssid": ap.ssid, "mhz": ap.mhz, "ch": ap.ch, "w": ap.width, "mine": i == mine}
		if i == mine {
			aps = append([]map[string]any{a}, aps...)
		} else {
			aps = append(aps, a)
		}
	}
	out["aps"] = aps
	if joined.halowMHz != nil {
		out["halowch"] = map[string]any{"mhz": joined.halowMHz, "w": joined.halowBW, "pw": joined.halowPBW, "ch": joined.halowCh}
	}
	if v.gate == nil {
		out["note"] = "The gate has no reading yet."
		writeJSON(w, out)
		return
	}
	names, complete := v.chain(joined, v.gate.mac)
	out["chain"] = names
	out["complete"] = complete
	out["hops"] = len(names) - 1
	if r := routeTo(joined, v.gate.mac); r != nil && joined != v.gate {
		if p := peerOf(joined, r.next); p != nil {
			out["halow"] = p.signal
		}
		out["tp"] = r.tp
	}
	writeJSON(w, out)
}

func (c *collector) meshHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("live") != "" {
		watchers.touch()
	}
	c.fresh(10 * time.Second)
	v := c.view()
	type link struct {
		N  string   `json:"n"`
		S  *float64 `json:"s"`
		TX *float64 `json:"tx"`
		RX *float64 `json:"rx"`
		L  string   `json:"l"`
	}
	type node struct {
		N        string   `json:"n"`
		IP       string   `json:"ip"`
		Role     string   `json:"role"`
		Model    string   `json:"model,omitempty"`
		Txp      *float64 `json:"txp,omitempty"`
		Err      string   `json:"err,omitempty"`
		Links    []link   `json:"links"`
		Route    []string `json:"route,omitempty"`
		Complete bool     `json:"complete"`
		TP       *float64 `json:"tp,omitempty"`
		Age      int      `json:"age"`
	}
	nodes := []node{}
	for _, n := range v.nodes {
		nd := node{N: n.host, IP: n.ip, Role: n.role, Model: n.model, Txp: n.txp, Err: n.err, Links: []link{}, Age: int(time.Since(n.at).Seconds())}
		for _, p := range n.peers {
			nd.Links = append(nd.Links, link{N: v.name(p.mac), S: p.signal, TX: p.txMbps, RX: p.rxMbps, L: p.plink})
		}
		if v.gate != nil && n.mac != "" && n != v.gate && n.err == "" {
			// The node's own route toward the gate, from its own table.
			nd.Route, nd.Complete = v.chain(n, v.gate.mac)
			if r := routeTo(n, v.gate.mac); r != nil {
				nd.TP = r.tp
			}
		}
		nodes = append(nodes, nd)
	}
	writeJSON(w, map[string]any{"nodes": nodes})
}

func readLeases() [][2]string { // [mac, ip]
	f, err := os.Open("/tmp/dhcp.leases")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out [][2]string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) >= 3 {
			out = append(out, [2]string{strings.ToLower(fs[1]), fs[2]})
		}
	}
	return out
}

func readARP() map[string]string { // ip -> mac
	out := map[string]string{}
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan() // header
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) >= 6 && fs[2] == "0x2" {
			out[fs[0]] = strings.ToLower(fs[3])
		}
	}
	return out
}

// readBatHosts names MACs from alfred's /etc/bat-hosts, e.g. "blue_wlh0" -> "blue".
// Node names by HaLow MAC, for nodes whose agent is not answering right now.
// Sources, later ones winning: alfred's /etc/bat-hosts ("green_wlh0" becomes
// "green"), /etc/haven-names ("<mac> <name>" lines, written by hand), and the
// names the console has learned from agents (namesFile).
var namesFile = "/etc/haven-console/names"

var learned struct {
	sync.Mutex
	names map[string]string
}

func readNames(path string, stripIface bool, out map[string]string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 2 || strings.HasPrefix(fs[0], "#") {
			continue
		}
		name := fs[1]
		if i := strings.LastIndex(name, "_"); stripIface && i > 0 {
			name = name[:i]
		}
		out[strings.ToLower(fs[0])] = name
	}
}

func readBatHosts() map[string]string {
	out := map[string]string{}
	readNames("/etc/bat-hosts", true, out)
	readNames("/etc/haven-names", false, out)
	learned.Lock()
	defer learned.Unlock()
	if learned.names == nil {
		learned.names = map[string]string{}
		readNames(namesFile, false, learned.names)
	}
	for mac, n := range learned.names {
		out[mac] = n
	}
	return out
}

// learnName remembers a node's name from its agent, and saves it when new.
func learnName(mac, host string) {
	if mac == "" || host == "" {
		return
	}
	readBatHosts() // loads the saved names once
	learned.Lock()
	defer learned.Unlock()
	if learned.names[mac] == host {
		return
	}
	learned.names[mac] = host
	var b strings.Builder
	for m, n := range learned.names {
		fmt.Fprintf(&b, "%s %s\n", m, n)
	}
	_ = os.WriteFile(namesFile, []byte(b.String()), 0o644)
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}

func init() { log.SetFlags(log.LstdFlags) }

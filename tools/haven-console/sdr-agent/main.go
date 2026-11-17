// The Parallel Toolchain's SDR agent (haven-sdr). It runs on a Haven node
// that has an RTL-SDR, like the node agent, and streams waterfall rows for the
// Toolchain's SDR tab: a sweep of a whole band to find active signals, and a
// zoomed view fast enough to read LoRa chirps (bandwidth and spreading
// factor). It has no page of its own; the gate passes its data to the tab.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"log"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	listen := flag.String("listen", ":8090", "address to serve on")
	rate := flag.Int("rate", 2048000, "sample rate in samples per second")
	port := flag.Int("rtl-port", 1234, "local port for rtl_tcp")
	pause := flag.String("pause", "/etc/init.d/adsbcot stop; /etc/init.d/dump1090 stop", "commands to run before taking the SDR")
	resume := flag.String("resume", "/etc/init.d/dump1090 start; /etc/init.d/adsbcot start", "commands to run after giving it back")
	idle := flag.Duration("idle", 30*time.Second, "release the SDR after this long with no viewer")
	flag.Parse()

	r := &radio{rate: uint32(*rate), port: *port, pause: *pause, resume: *resume, idle: *idle}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Parallel Toolchain SDR agent: open the SDR tab in the Toolchain.", http.StatusNotFound)
	})
	mux.HandleFunc("/sdr/status", r.status)
	mux.HandleFunc("/sdr/zoom", r.zoom)
	mux.HandleFunc("/sdr/sweep", r.sweep)
	mux.HandleFunc("/sdr/identify", r.identify)
	log.Printf("haven-sdr on %s (%d samples/s)", *listen, *rate)
	log.Fatal(http.ListenAndServe(*listen, mux))
}

func (r *radio) status(w http.ResponseWriter, req *http.Request) {
	out, _ := exec.Command("sh", "-c", "lsusb 2>/dev/null | grep -i -E 'rtl28|0bda:2838|rtl-sdr'").Output()
	r.mu.Lock()
	busy := r.conn != nil
	inUse := r.viewers > 0
	r.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{
		"sdr": strings.TrimSpace(string(out)) != "", "device": strings.TrimSpace(string(out)),
		"busy": busy, "inUse": inUse, "rate": r.rate,
	})
}

func num(req *http.Request, key string, def float64) float64 {
	if v, err := strconv.ParseFloat(req.URL.Query().Get(key), 64); err == nil {
		return v
	}
	return def
}

// rowWriter streams fixed-size rows and flushes about 20 times a second.
type rowWriter struct {
	w     http.ResponseWriter
	f     http.Flusher
	last  time.Time
	alive func() bool
}

func (rw *rowWriter) row(b []byte) bool {
	if _, err := rw.w.Write(b); err != nil {
		return false
	}
	if time.Since(rw.last) > 50*time.Millisecond {
		rw.f.Flush()
		rw.last = time.Now()
	}
	return rw.alive()
}

func (r *radio) begin(w http.ResponseWriter, req *http.Request, hdr map[string]string) (int64, *bufio.Reader, *rowWriter, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return 0, nil, nil, false
	}
	id, rd, err := r.take()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return 0, nil, nil, false
	}
	r.gain(num(req, "gain", 0))
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	for k, v := range hdr {
		h.Set(k, v)
	}
	w.WriteHeader(200)
	f.Flush()
	ctx := req.Context()
	alive := func() bool { return ctx.Err() == nil && r.current(id) }
	return id, rd, &rowWriter{w: w, f: f, alive: alive}, true
}

// zoom streams a live waterfall around one frequency. Each row is the peak
// power per bin over rowMs milliseconds, one byte per bin.
func (r *radio) zoom(w http.ResponseWriter, req *http.Request) {
	center := uint32(num(req, "f", 923e6))
	bins := pow2(int(num(req, "bins", 512)), 64, 4096)
	rowMs := math.Max(1, math.Min(100, num(req, "ms", 5)))
	ffts := int(math.Max(1, math.Round(float64(r.rate)*rowMs/1000/float64(bins))))
	rowMs = float64(ffts*bins) / float64(r.rate) * 1000 // what it really is

	id, rd, rw, ok := r.begin(w, req, map[string]string{
		"X-SDR-Mode": "zoom", "X-SDR-Center": strconv.Itoa(int(center)), "X-SDR-Rate": strconv.Itoa(int(r.rate)),
		"X-SDR-Bins": strconv.Itoa(bins), "X-SDR-Row-Ms": strconv.FormatFloat(rowMs, 'f', 4, 64),
	})
	if !ok {
		return
	}
	defer r.done(id)
	r.tune(center)
	if catchUp(rd, r.rate) != nil || discard(rd, r.rate, 150*time.Millisecond) != nil {
		return
	}

	// Read the radio at its own pace and hand rows to the writer. If the
	// viewer falls behind, drop rows instead of falling behind real time.
	rows := make(chan []byte, 128)
	quit := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer close(rows)
		s := newSpectrum(bins)
		iq := make([]byte, 2*bins)
		pw := make([]float64, bins)
		for {
			for i := range pw {
				pw[i] = 0
			}
			for k := 0; k < ffts; k++ {
				if _, err := io.ReadFull(rd, iq); err != nil {
					return
				}
				s.power(iq, pw)
			}
			out := make([]byte, bins)
			toBytes(pw, out)
			select {
			case <-quit:
				return
			case rows <- out:
			default: // viewer is behind: drop this row
			}
		}
	}()
	defer func() { close(quit); <-finished }() // stop reading before giving the SDR back
	for out := range rows {
		if !rw.row(out) {
			return
		}
	}
}

// sweep streams a waterfall of a whole band. The tuner hops across it in
// steps; each row is one pass, built from the middle of each step.
func (r *radio) sweep(w http.ResponseWriter, req *http.Request) {
	start := num(req, "start", 902e6)
	stop := num(req, "stop", 928e6)
	bins := pow2(int(num(req, "bins", 256)), 64, 2048) // per step
	dwell := time.Duration(math.Max(10, math.Min(2000, num(req, "dwell", 60)))) * time.Millisecond
	use := bins * 3 / 4 // the outer edges of each step roll off; skip them
	stepHz := float64(r.rate) * float64(use) / float64(bins)
	steps := int(math.Ceil((stop - start) / stepHz))
	width := steps * use
	binHz := float64(r.rate) / float64(bins)

	id, rd, rw, ok := r.begin(w, req, map[string]string{
		"X-SDR-Mode": "sweep", "X-SDR-Start": strconv.Itoa(int(start)),
		"X-SDR-Stop": strconv.Itoa(int(start + float64(width)*binHz)),
		"X-SDR-Bins": strconv.Itoa(width), "X-SDR-Steps": strconv.Itoa(steps), "X-SDR-Use": strconv.Itoa(use),
		"X-SDR-Row-Ms": strconv.Itoa(int((dwell + 120*time.Millisecond).Milliseconds()) * steps),
	})
	if !ok {
		return
	}
	defer r.done(id)
	if catchUp(rd, r.rate) != nil {
		return
	}
	s := newSpectrum(bins)
	iq := make([]byte, 2*bins)
	pw := make([]float64, bins)
	rec := make([]byte, 2+use)
	part := make([]byte, bins)
	ffts := int(math.Max(1, float64(r.rate)*dwell.Seconds()/float64(bins)))
	blocks := make([][]float64, ffts)
	for k := range blocks {
		blocks[k] = make([]float64, bins)
	}
	medians := make([]float64, ffts)
	for {
		for st := 0; st < steps; st++ {
			center := start + stepHz*float64(st) + stepHz/2
			r.tune(uint32(center))
			if discard(rd, r.rate, 120*time.Millisecond) != nil {
				return
			}
			for k := 0; k < ffts; k++ {
				if _, err := io.ReadFull(rd, iq); err != nil {
					return
				}
				medians[k] = s.block(iq, blocks[k])
			}
			// A strong transmitter nearby (like this Haven's own HaLow radio)
			// can overload the receiver and lift the whole step for a moment.
			// Keep the peak only from blocks whose overall level stays within
			// 10 dB of this step's quiet level, so overload isn't painted as
			// signals.
			quiet := percentile(medians, 0.25)
			for i := range pw {
				pw[i] = 0
			}
			for k := 0; k < ffts; k++ {
				if medians[k] > quiet*10 {
					continue
				}
				for i, v := range blocks[k] {
					if v > pw[i] {
						pw[i] = v
					}
				}
			}
			toBytes(pw, part)
			// Send each step as soon as it's measured, so the page draws the
			// scan as it goes: 2 bytes of step number, then its bins.
			rec[0], rec[1] = byte(st>>8), byte(st)
			copy(rec[2:], part[(bins-use)/2:(bins-use)/2+use])
			if !rw.row(rec) {
				return
			}
			rw.f.Flush()
		}
	}
}

func pow2(v, lo, hi int) int {
	n := lo
	for n < v && n < hi {
		n <<= 1
	}
	return n
}

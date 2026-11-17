package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// radio owns the RTL-SDR through rtl_tcp. Only one view can use it at a time;
// a new view takes over from the old one. While it's in use, other services
// that need the SDR (ADS-B) are paused, and they come back after it has been
// idle for a while.
type radio struct {
	rate   uint32
	port   int
	pause  string // shell command run before taking the SDR
	resume string // shell command run after giving it back
	idle   time.Duration

	mu      sync.Mutex
	proc    *exec.Cmd
	conn    net.Conn
	rd      *bufio.Reader
	paused  bool
	session int64
	timer   *time.Timer
	viewers int // views currently reading samples (0 or 1)

	readMu sync.Mutex // held by the view that is reading samples
}

// take stops any other view, makes sure rtl_tcp is running, and returns a
// session id plus the sample reader. Call done(id) when finished.
func (r *radio) take() (int64, *bufio.Reader, error) {
	r.mu.Lock()
	r.session++
	id := r.session
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	r.mu.Unlock()

	r.readMu.Lock() // waits for the previous view to notice and stop

	r.mu.Lock()
	defer r.mu.Unlock()
	r.viewers++
	if !r.paused && r.pause != "" {
		run(r.pause)
	}
	r.paused = true
	if r.conn == nil {
		if err := r.start(); err != nil {
			r.viewers--
			r.readMu.Unlock()
			return 0, nil, err
		}
	}
	return id, r.rd, nil
}

func (r *radio) current(id int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.session == id
}

// done ends a view. If nothing else takes the SDR within the idle time, it
// stops rtl_tcp and resumes the paused services.
func (r *radio) done(id int64) {
	r.mu.Lock()
	r.viewers--
	r.mu.Unlock()
	r.readMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session != id {
		return
	}
	if r.timer != nil {
		r.timer.Stop()
	}
	r.timer = time.AfterFunc(r.idle, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.session != id {
			return
		}
		r.stop()
		if r.paused && r.resume != "" {
			run(r.resume)
		}
		r.paused = false
		log.Printf("idle: SDR released")
	})
}

func (r *radio) start() error {
	// Clear out an rtl_tcp left over from a previous run of this program; it
	// would hold the SDR.
	exec.Command("killall", "rtl_tcp").Run()
	time.Sleep(300 * time.Millisecond)

	r.proc = exec.Command("rtl_tcp", "-a", "127.0.0.1", "-p", strconv.Itoa(r.port), "-s", strconv.Itoa(int(r.rate)))
	var msgs strings.Builder
	r.proc.Stdout, r.proc.Stderr = &msgs, &msgs
	if err := r.proc.Start(); err != nil {
		r.proc = nil
		return fmt.Errorf("rtl_tcp: %w", err)
	}
	exited := make(chan struct{})
	go func(p *exec.Cmd) { p.Wait(); close(exited) }(r.proc)
	var err error
	for i := 0; i < 100; i++ { // up to 15 s: the tuner can be slow to come up
		select {
		case <-exited:
			r.proc = nil
			log.Printf("rtl_tcp exited: %s", msgs.String())
			return fmt.Errorf("rtl_tcp exited (is the RTL-SDR plugged in, or in use?)")
		case <-time.After(150 * time.Millisecond):
		}
		r.conn, err = net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", r.port))
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Printf("rtl_tcp did not answer: %s", msgs.String())
		r.stop()
		return fmt.Errorf("rtl_tcp did not start in time: %w", err)
	}
	r.rd = bufio.NewReaderSize(r.conn, 1<<20)
	head := make([]byte, 12) // "RTL0", tuner type, gain count
	if _, err := io.ReadFull(r.rd, head); err != nil || string(head[:4]) != "RTL0" {
		r.stop()
		return fmt.Errorf("rtl_tcp: no RTL-SDR found")
	}
	log.Printf("SDR started (tuner type %d)", binary.BigEndian.Uint32(head[4:8]))
	return nil
}

func (r *radio) stop() {
	if r.conn != nil {
		r.conn.Close()
		r.conn = nil
	}
	if r.proc != nil && r.proc.Process != nil {
		r.proc.Process.Kill() // the goroutine started in start() reaps it
	}
	r.proc = nil
}

// command sends one rtl_tcp control command: 1 byte + 32-bit big-endian value.
func (r *radio) command(cmd byte, v uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn == nil {
		return
	}
	b := []byte{cmd, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(b[1:], v)
	r.conn.Write(b)
}

func (r *radio) tune(hz uint32) { r.command(0x01, hz) }

// gain sets the tuner gain in dB (0 and up; the tuner uses its nearest
// supported step), or automatic gain for a negative value.
func (r *radio) gain(db float64) {
	if db < 0 || math.IsNaN(db) {
		r.command(0x03, 0)
		return
	}
	r.command(0x03, 1)
	r.command(0x04, uint32(math.Round(math.Min(db, 60)*10)))
}

// catchUp drops samples that piled up while nothing was reading (rtl_tcp keeps
// a backlog), so a view starts on live samples. Backlogged data arrives much
// faster than real time; once 100 ms of reading brings in less than 1.5 times
// what the radio makes in 100 ms, we're live.
func catchUp(rd *bufio.Reader, rate uint32) error {
	chunk := make([]byte, 16*1024)
	live := int(float64(rate) * 2 * 0.1 * 1.5)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		start, n := time.Now(), 0
		for time.Since(start) < 100*time.Millisecond {
			k, err := io.ReadFull(rd, chunk)
			if err != nil {
				return err
			}
			n += k
		}
		if n < live {
			return nil
		}
	}
	return nil
}

// discard drops the samples still in flight from before a retune.
func discard(rd *bufio.Reader, rate uint32, d time.Duration) error {
	n := int64(float64(rate) * 2 * d.Seconds())
	_, err := io.CopyN(io.Discard, rd, n)
	return err
}

func run(cmd string) {
	for _, c := range strings.Split(cmd, ";") {
		if c = strings.TrimSpace(c); c != "" {
			exec.Command("/bin/sh", "-c", c).Run()
		}
	}
}

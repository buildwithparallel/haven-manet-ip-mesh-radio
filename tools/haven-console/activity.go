package main

// Nothing polls the mesh unless someone is looking. The gate's background
// work (reading every node's agent, tracing routes) sends traffic over HaLow,
// so it runs only while a page is asking for live data, and stops 20 seconds
// after the last request.

import (
	"sync"
	"time"
)

const idleAfter = 20 * time.Second

type activity struct {
	mu   sync.Mutex
	last time.Time
}

var watchers activity

// touch records that someone is looking at live data now.
func (a *activity) touch() {
	a.mu.Lock()
	a.last = time.Now()
	a.mu.Unlock()
}

func (a *activity) active() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Since(a.last) < idleAfter
}

// wait blocks until someone is looking. It reports whether it had to wait,
// meaning the work it guards was idle and its state may be old.
func (a *activity) wait() (wasIdle bool) {
	for !a.active() {
		wasIdle = true
		time.Sleep(250 * time.Millisecond)
	}
	return wasIdle
}

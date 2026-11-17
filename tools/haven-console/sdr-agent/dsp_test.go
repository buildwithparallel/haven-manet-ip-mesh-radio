package main

import (
	"math"
	"testing"
)

// A tone at +f from the centre must land f/(rate/n) bins right of the middle,
// and a full-scale tone must read close to 0 dBFS.
func TestToneLandsInRightBin(t *testing.T) {
	const n, rate = 512, 2048000.0
	for _, off := range []float64{-300e3, 125e3, 600e3} { // not 0 Hz: that's removed as DC, below
		iq := make([]byte, 2*n)
		for i := 0; i < n; i++ {
			ph := 2 * math.Pi * off * float64(i) / rate
			iq[2*i] = byte(127.5 + 127*math.Cos(ph))
			iq[2*i+1] = byte(127.5 + 127*math.Sin(ph))
		}
		pw := make([]float64, n)
		newSpectrum(n).power(iq, pw)
		best := 0
		for i := range pw {
			if pw[i] > pw[best] {
				best = i
			}
		}
		want := n/2 + int(math.Round(off/(rate/n)))
		if best != want {
			t.Errorf("tone at %+.0f Hz: peak in bin %d, want %d", off, best, want)
		}
		if db := 10 * math.Log10(pw[best]); db < -2 || db > 1 {
			t.Errorf("tone at %+.0f Hz: %.1f dBFS, want about 0", off, db)
		}
	}
}

// The RTL-SDR's constant offset (DC) must not show as a spike at the centre.
func TestDCRemoved(t *testing.T) {
	const n = 512
	iq := make([]byte, 2*n)
	for i := range iq {
		iq[i] = 140 // offset, no signal
	}
	pw := make([]float64, n)
	newSpectrum(n).power(iq, pw)
	if pw[n/2] > 1e-9 {
		t.Errorf("DC bin %.3g, want ~0", pw[n/2])
	}
}

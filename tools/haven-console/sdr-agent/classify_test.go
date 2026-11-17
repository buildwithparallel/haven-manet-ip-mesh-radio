package main

import (
	"math"
	"math/rand"
	"testing"
)

// synth makes 8-bit I/Q: noise, plus a signal from s0 to s1 given by phase(k).
func synth(fs float64, n, s0, s1 int, amp float64, phase func(k int) float64) []byte {
	rng := rand.New(rand.NewSource(1))
	raw := make([]byte, 2*n)
	for k := 0; k < n; k++ {
		i, q := rng.NormFloat64()*4, rng.NormFloat64()*4
		if k >= s0 && k < s1 {
			p := phase(k - s0)
			i += amp * math.Cos(p)
			q += amp * math.Sin(p)
		}
		raw[2*k] = byte(math.Max(0, math.Min(255, 127.5+i)))
		raw[2*k+1] = byte(math.Max(0, math.Min(255, 127.5+q)))
	}
	return raw
}

// lora: upchirps (random symbol values) of bandwidth bw and spreading
// factor sf, offset off Hz from the tuned frequency.
func lora(fs, off, bw float64, sf int) func(int) float64 {
	m := math.Pow(2, float64(sf))
	ts := m / bw
	rng := rand.New(rand.NewSource(2))
	sym := map[int]float64{}
	return func(k int) float64 {
		t := float64(k) / fs
		i := int(t / ts)
		if _, ok := sym[i]; !ok {
			if i < 8 {
				sym[i] = 0 // preamble
			} else {
				sym[i] = float64(rng.Intn(int(m)))
			}
		}
		tau := t - float64(i)*ts
		// cyclically shifted chirp: frequency starts at -bw/2 + shift
		f0 := -bw/2 + sym[i]/m*bw
		tw := (bw/2 - f0) / (bw / ts) // time until it wraps
		ph := 0.0
		if tau < tw {
			ph = 2 * math.Pi * (f0*tau + bw/(2*ts)*tau*tau)
		} else {
			ph = 2*math.Pi*(f0*tw+bw/(2*ts)*tw*tw) + 2*math.Pi*(-bw/2*(tau-tw)+bw/(2*ts)*(tau-tw)*(tau-tw))
		}
		return ph + 2*math.Pi*off*t + float64(i)*1.234
	}
}

func TestIdentifyLoRa(t *testing.T) {
	fs := 2.048e6
	for _, c := range []struct {
		bw float64
		sf int
	}{{125e3, 7}, {125e3, 9}, {250e3, 11}, {500e3, 8}} {
		dur := int(math.Pow(2, float64(c.sf)) / c.bw * 30 * fs) // 30 symbols
		n := dur + int(0.3*fs)
		raw := synth(fs, n, int(0.1*fs), int(0.1*fs)+dur, 30, lora(fs, 300e3, c.bw, c.sf))
		bs := analyze(raw, fs, 915e6)
		found := false
		for _, b := range bs {
			t.Logf("bw %.0f SF%d -> %+v", c.bw, c.sf, b)
			if b.Kind == "lora" && b.BW == c.bw && b.SF == c.sf && math.Abs(b.CenterHz-915.3e6) < 20e3 {
				found = true
			}
		}
		if !found {
			t.Errorf("LoRa %.0f kHz SF%d not identified", c.bw/1e3, c.sf)
		}
	}
}

func TestIdentifyFSK(t *testing.T) {
	fs := 2.048e6
	rng := rand.New(rand.NewSource(3))
	bits := make([]float64, 4000)
	for i := range bits {
		bits[i] = float64(rng.Intn(2)*2 - 1)
	}
	dev, baud := 25e3, 50e3
	phase := 0.0
	last := -1
	fsk := func(k int) float64 {
		if k != last+1 {
			phase = 0
		}
		last = k
		f := -400e3 + bits[int(float64(k)/fs*baud)%len(bits)]*dev
		phase += 2 * math.Pi * f / fs
		return phase
	}
	n := int(0.2 * fs)
	raw := synth(fs, n, int(0.05*fs), int(0.15*fs), 30, fsk)
	bs := analyze(raw, fs, 915e6)
	ok := false
	for _, b := range bs {
		t.Logf("%+v", b)
		if b.Kind == "fsk" && math.Abs(b.DevHz-dev) < 8e3 {
			ok = true
		}
	}
	if !ok {
		t.Error("FSK not identified")
	}
}

// A signal sitting on the tuned frequency must not be split by the skipped
// DC bins into pieces (which then misread as FSK or as separate signals).
func TestIdentifyLoRaOnCenter(t *testing.T) {
	fs := 2.048e6
	bw, sf := 500e3, 7
	dur := int(math.Pow(2, float64(sf)) / bw * 300 * fs) // ~77 ms, like a long packet
	raw := synth(fs, dur+int(0.3*fs), int(0.1*fs), int(0.1*fs)+dur, 30, lora(fs, -95e3, bw, sf))
	bs := analyze(raw, fs, 915e6)
	lora, other := 0, 0
	for _, b := range bs {
		t.Logf("%+v", b)
		if b.Kind == "lora" && b.BW == bw && b.SF == sf && math.Abs(b.CenterHz-914.905e6) < 30e3 {
			lora++
		} else if b.Kind != "tone" {
			other++
		}
	}
	if lora != 1 || other != 0 {
		t.Errorf("want one LoRa burst near 914.905 MHz and nothing else; got %d LoRa, %d other", lora, other)
	}
}

// A LoRa radio right next to the SDR clips the 8-bit samples and smears
// across the view; it must still be identified as LoRa, not "wideband".
func TestIdentifyLoRaOverloaded(t *testing.T) {
	fs := 2.048e6
	bw, sf := 500e3, 7
	dur := int(math.Pow(2, float64(sf)) / bw * 300 * fs)
	raw := synth(fs, dur+int(0.3*fs), int(0.1*fs), int(0.1*fs)+dur, 400, lora(fs, -60e3, bw, sf)) // far past full scale
	bs := analyze(raw, fs, 915e6)
	ok := false
	for _, b := range bs {
		t.Logf("%+v", b)
		if b.Kind == "lora" && b.BW == bw && b.SF == sf {
			ok = true
		}
	}
	if !ok {
		t.Error("overloaded LoRa not identified")
	}
}

package main

import (
	"math"
	"math/cmplx"
	"sort"
)

// spectrum turns blocks of RTL-SDR samples (interleaved unsigned 8-bit I/Q)
// into power per frequency bin, in dB relative to full scale.
type spectrum struct {
	n       int
	window  []float64
	norm    float64 // 1 / (sum of window)^2
	twiddle []complex128
	rev     []int
	buf     []complex128
}

func newSpectrum(n int) *spectrum {
	s := &spectrum{n: n, window: make([]float64, n), twiddle: make([]complex128, n/2), rev: make([]int, n), buf: make([]complex128, n)}
	sum := 0.0
	for i := range s.window {
		s.window[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1)) // Hann
		sum += s.window[i]
	}
	s.norm = 1 / (sum * sum)
	for i := range s.twiddle {
		s.twiddle[i] = cmplx.Exp(complex(0, -2*math.Pi*float64(i)/float64(n)))
	}
	bits := 0
	for 1<<bits < n {
		bits++
	}
	for i := range s.rev {
		r := 0
		for b := 0; b < bits; b++ {
			if i&(1<<b) != 0 {
				r |= 1 << (bits - 1 - b)
			}
		}
		s.rev[i] = r
	}
	return s
}

// power adds this block's power per bin into out, keeping the maximum (peak
// hold), with DC in the middle: out[0] is the lowest frequency.
func (s *spectrum) power(iq []byte, out []float64) {
	n := s.n
	// Remove the block's average (DC). The RTL-SDR adds a small constant
	// offset that would otherwise show as a spike at the tuned frequency.
	var mi, mq float64
	for i := 0; i < n; i++ {
		mi += float64(iq[2*i])
		mq += float64(iq[2*i+1])
	}
	mi, mq = mi/float64(n), mq/float64(n)
	for i := 0; i < n; i++ {
		re := (float64(iq[2*i]) - mi) / 127.5
		im := (float64(iq[2*i+1]) - mq) / 127.5
		s.buf[s.rev[i]] = complex(re*s.window[i], im*s.window[i])
	}
	for size := 2; size <= n; size <<= 1 {
		half, step := size/2, n/size
		for start := 0; start < n; start += size {
			for k := 0; k < half; k++ {
				t := s.twiddle[k*step] * s.buf[start+k+half]
				u := s.buf[start+k]
				s.buf[start+k] = u + t
				s.buf[start+k+half] = u - t
			}
		}
	}
	for i := 0; i < n; i++ {
		v := s.buf[i]
		p := (real(v)*real(v) + imag(v)*imag(v)) * s.norm
		j := (i + n/2) % n // shift so the centre frequency sits in the middle
		if p > out[j] {
			out[j] = p
		}
	}
}

// block writes one block's power per bin into out (no peak hold) and returns
// the median, a measure of the block's overall level.
func (s *spectrum) block(iq []byte, out []float64) float64 {
	for i := range out {
		out[i] = 0
	}
	s.power(iq, out)
	return percentile(out, 0.5)
}

func percentile(v []float64, q float64) float64 {
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	return c[int(q*float64(len(c)-1))]
}

// toBytes maps power to one byte per bin: 0.5 dB steps from -110 dBFS.
func toBytes(p []float64, out []byte) {
	for i, v := range p {
		db := -150.0
		if v > 0 {
			db = 10 * math.Log10(v)
		}
		b := (db + 110) * 2
		if b < 0 {
			b = 0
		}
		if b > 255 {
			b = 255
		}
		out[i] = byte(b)
	}
}

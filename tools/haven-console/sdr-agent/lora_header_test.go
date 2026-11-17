package main

import (
	"math"
	"testing"
)

// headerChirps builds the 8 chirp ids of an explicit LoRa header, the same
// way a Semtech transmitter does: Hamming 4/8, interleave, parity bit, gray
// decode, then +1. The payload nibbles that share the first block are zeros.
func headerChirps(sf, cr, payload int, hasCRC bool) []int {
	nib := make([]int, sf-2)
	nib[0] = (payload >> 4) & 0xF
	nib[1] = payload & 0xF
	crcBit := 0
	if hasCRC {
		crcBit = 1
	}
	nib[2] = (cr << 1) | crcBit
	bit := func(v, i int) int { return (v >> i) & 1 }
	c4 := bit(nib[0], 3) ^ bit(nib[0], 2) ^ bit(nib[0], 1) ^ bit(nib[0], 0)
	c3 := bit(nib[0], 3) ^ bit(nib[1], 3) ^ bit(nib[1], 2) ^ bit(nib[1], 1) ^ bit(nib[2], 0)
	c2 := bit(nib[0], 2) ^ bit(nib[1], 3) ^ bit(nib[1], 0) ^ bit(nib[2], 3) ^ bit(nib[2], 1)
	c1 := bit(nib[0], 1) ^ bit(nib[1], 2) ^ bit(nib[1], 0) ^ bit(nib[2], 2) ^ bit(nib[2], 1) ^ bit(nib[2], 0)
	c0 := bit(nib[0], 0) ^ bit(nib[1], 1) ^ bit(nib[2], 3) ^ bit(nib[2], 2) ^ bit(nib[2], 1) ^ bit(nib[2], 0)
	nib[3] = c4
	nib[4] = c3<<3 | c2<<2 | c1<<1 | c0

	sfApp := sf - 2
	const cwLen = 8
	cws := make([]int, sfApp)
	for i := 0; i < sfApp && i < len(nib); i++ {
		cws[i] = hammingEncode(nib[i])
	}
	symbols := make([]int, cwLen)
	for i := 0; i < cwLen; i++ {
		word := 0
		for j := 0; j < sfApp; j++ {
			src := mod(i-j-1, sfApp)
			word = (word << 1) | ((cws[src] >> (cwLen - 1 - i)) & 1)
		}
		parity := 0
		for tmp := word; tmp > 0; tmp >>= 1 {
			parity ^= tmp & 1
		}
		s := (word << 2) | (parity << 1)
		g := s
		for j := 1; j < sf; j++ {
			g ^= s >> j
		}
		symbols[i] = mod(g+1, 1<<sf)
	}
	return symbols
}

func hammingEncode(nib int) int {
	d := [4]int{(nib >> 3) & 1, (nib >> 2) & 1, (nib >> 1) & 1, nib & 1}
	p0 := d[3] ^ d[2] ^ d[1]
	p1 := d[2] ^ d[1] ^ d[0]
	p2 := d[3] ^ d[2] ^ d[0]
	p3 := d[3] ^ d[1] ^ d[0]
	return d[3]<<7 | d[2]<<6 | d[1]<<5 | d[0]<<4 | p0<<3 | p1<<2 | p2<<1 | p3
}

// frameIQ writes one explicit-header packet at headerOS samples per chip.
func frameIQ(sf, cr, preamble int, off float64) []complex128 {
	nSym := 1 << sf
	sym := headerOS * nSym
	ids := headerChirps(sf, cr, 12, true)
	// preamble upchirps, two sync symbols, 2.25 downchirps, then the header
	n := (preamble+2)*sym + 9*sym/4 + len(ids)*sym
	up0 := upchirp(0, sf, headerOS)
	down := make([]complex128, len(up0))
	for i, v := range up0 {
		down[i] = complex(real(v), -imag(v))
	}
	sync0 := upchirp(((0x12&0xF0)>>4)<<3, sf, headerOS)
	sync1 := upchirp((0x12&0x0F)<<3, sf, headerOS)
	out := make([]complex128, n+sym) // a little tail, like a payload symbol
	put := func(at int, w []complex128) {
		for i, v := range w {
			if at+i < len(out) {
				out[at+i] = v
			}
		}
	}
	for i := 0; i < preamble; i++ {
		put(i*sym, up0)
	}
	put(preamble*sym, sync0)
	put((preamble+1)*sym, sync1)
	put((preamble+2)*sym, down)
	put((preamble+3)*sym, down)
	put((preamble+4)*sym, down[:sym/4])
	for i, id := range ids {
		put((preamble+4)*sym+sym/4+i*sym, upchirp(id, sf, headerOS))
	}
	if off != 0 {
		step := 2 * math.Pi * off / (float64(headerOS) * 500e3) // caller passes Hz at bw 500; tests at other bw scale off
		for i, v := range out {
			ph := step * float64(i)
			out[i] = v * complex(math.Cos(ph), math.Sin(ph))
		}
	}
	return out
}

func TestCodingRateRoundtrip(t *testing.T) {
	for _, c := range []struct {
		sf, cr, pre int
	}{
		{7, 1, 8},
		{7, 2, 8},
		{7, 3, 12},
		{7, 4, 8},
		{8, 1, 8},
		{9, 4, 8},
		{11, 2, 8},
	} {
		x := frameIQ(c.sf, c.cr, c.pre, 0)
		// A few samples of silence in front, the way a burst detector starts early.
		pad := make([]complex128, headerOS*(1<<c.sf)/3)
		x = append(pad, x...)
		got := crFromBaseband(x, c.sf)
		if got != c.cr {
			t.Errorf("SF%d preamble %d: coding rate 4/%d read as %d", c.sf, c.pre, 4+c.cr, got)
		}
	}
}

func TestIdentifyCodingRate(t *testing.T) {
	fs := 2.048e6
	for _, c := range []struct {
		bw  float64
		sf  int
		cr  int
		off float64
		pre int
	}{
		{500e3, 7, 1, 80e3, 8},
		{500e3, 7, 3, -120e3, 8},
		{250e3, 9, 2, 40e3, 8},
		{125e3, 8, 4, 30e3, 16},
	} {
		sig := packetAt(fs, c.off, c.bw, c.sf, c.cr, c.pre)
		lead := int(0.05 * fs)
		n := lead + len(sig) + int(0.05*fs)
		raw := synth(fs, n, lead, lead+len(sig), 40, func(k int) float64 {
			if k < 0 || k >= len(sig) {
				return 0
			}
			return sig[k]
		})
		bs := analyze(raw, fs, 915e6)
		found := false
		for _, b := range bs {
			t.Logf("bw %.0f SF%d CR 4/%d -> %+v", c.bw, c.sf, 4+c.cr, b)
			if b.Kind == "lora" && b.BW == c.bw && b.SF == c.sf && b.CR == c.cr {
				found = true
			}
		}
		if !found {
			t.Errorf("LoRa %.0f kHz SF%d coding rate 4/%d not identified", c.bw/1e3, c.sf, 4+c.cr)
		}
	}
}

// packetAt returns the phase of each sample of one packet at sample rate fs.
// The result is a phase sequence indexed by sample, stored as the return of
// the closure captured below; this helper instead returns the phases so synth
// can play them. Phases longer than the packet are 0.
func packetAt(fs, off, bw float64, sf, cr, preamble int) []float64 {
	nSym := float64(int(1) << uint(sf))
	ids := headerChirps(sf, cr, 12, true)
	// chips: preamble + 2 sync + 2.25 down + header
	chips := (float64(preamble)+4.25)*nSym + float64(len(ids))*nSym
	n := int(chips * fs / bw)
	ph := make([]float64, n)
	for k := 0; k < n; k++ {
		chip := float64(k) * bw / fs
		ph[k] = framePhase(chip, nSym, sf, preamble, ids) + 2*math.Pi*off*float64(k)/fs
	}
	return ph
}

func framePhase(chip, nSym float64, sf, preamble int, ids []int) float64 {
	pre := float64(preamble) * nSym
	switch {
	case chip < pre:
		return chirpPhase(0, sf, math.Mod(chip, nSym))
	case chip < pre+nSym:
		return chirpPhase(((0x12&0xF0)>>4)<<3, sf, chip-pre)
	case chip < pre+2*nSym:
		return chirpPhase((0x12&0x0F)<<3, sf, chip-pre-nSym)
	case chip < pre+4.25*nSym:
		return -chirpPhase(0, sf, math.Mod(chip-(pre+2*nSym), nSym))
	default:
		into := chip - (pre + 4.25*nSym)
		i := int(into / nSym)
		if i >= len(ids) {
			i = len(ids) - 1
		}
		return chirpPhase(ids[i], sf, into-float64(i)*nSym)
	}
}

func chirpPhase(id, sf int, n float64) float64 {
	N := float64(int(1) << uint(sf))
	fold := N - float64(id)
	if n < fold {
		return 2 * math.Pi * (n*n/(2*N) + (float64(id)/N-0.5)*n)
	}
	return 2 * math.Pi * (n*n/(2*N) + (float64(id)/N-1.5)*n)
}

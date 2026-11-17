package main

// The LoRa coding rate is not in the shape of a chirp. Bandwidth and spreading
// factor fix that. The rate is three bits in the explicit header that
// Meshtastic, Reticulum, and LoRaWAN send ahead of the payload.
//
// The header is always encoded at coding rate 4/8, then interleaved across one
// block of symbols. This follows the same steps as a LoRa receiver: find the
// preamble and the downchirps that end it, demodulate the header symbols, undo
// the gray mapping and the interleave, Hamming-decode, and keep the rate only
// when the header checksum matches.

import (
	"math"
	"math/cmplx"
)

// headerOS is the samples per chip used while reading a header. Two leaves
// the chirp inside the Nyquist frequency, so resampling doesn't clip it.
const headerOS = 2

// codingRate returns 1..4 (coding rates 4/5..4/8), or 0 when no header checks out.
func codingRate(raw []byte, fs, off, bw float64, sf, s0, s1 int) int {
	if sf < 7 || sf > 12 || bw <= 0 || s1-s0 < 64 {
		return 0
	}
	x := resampleBaseband(raw, fs, off, bw*headerOS, s0, s1)
	return crFromBaseband(x, sf)
}

// crFromBaseband reads the coding rate from samples at headerOS samples per
// chip, with the burst already shifted to 0 Hz.
func crFromBaseband(x []complex128, sf int) int {
	nSym := 1 << sf
	sym := headerOS * nSym // samples in one symbol
	if len(x) < 16*sym {
		return 0
	}
	// The header sits right after the preamble. Keep that part and no more.
	if max := 64 * sym; len(x) > max {
		x = x[:max]
	}
	up := upchirp(0, sf, headerOS)
	down := make([]complex128, len(up))
	for i, v := range up {
		down[i] = cmplx.Conj(v)
	}
	buf := make([]complex128, sym)
	off := bestTiming(x, down, sym, buf)
	if off < 0 {
		return 0
	}
	sfd, cfo := findSFD(x, up, down, sym, off, buf)
	if sfd < 0 {
		return 0
	}
	// Two full downchirps and a quarter, then the header. The quarter is what
	// puts the header a quarter symbol off the preamble's grid.
	start := off + sfd*sym + 9*sym/4
	for _, c := range []int{cfo, 0} {
		if cr, ok := headerAt(x, sf, start, c, down, buf); ok {
			return cr
		}
	}
	return 0
}

// resampleBaseband shifts the burst to 0 Hz and linearly resamples it to rate.
func resampleBaseband(raw []byte, fs, off, rate float64, s0, s1 int) []complex128 {
	nIn := s1 - s0
	if nIn < 8 {
		return nil
	}
	var mean complex128
	for k := s0; k < s1; k++ {
		mean += iqAt(raw, k)
	}
	mean /= complex(float64(nIn), 0)
	nOut := int(float64(nIn) * rate / fs)
	out := make([]complex128, nOut)
	step := -2 * math.Pi * off / fs
	ratio := fs / rate
	for i := 0; i < nOut; i++ {
		pos := float64(i) * ratio
		j := int(pos)
		if j >= nIn-1 {
			break
		}
		frac := pos - float64(j)
		ph0 := step * float64(j)
		ph1 := step * float64(j+1)
		a := (iqAt(raw, s0+j) - mean) * complex(math.Cos(ph0), math.Sin(ph0))
		b := (iqAt(raw, s0+j+1) - mean) * complex(math.Cos(ph1), math.Sin(ph1))
		out[i] = a*complex(1-frac, 0) + b*complex(frac, 0)
	}
	return out
}

// upchirp is one LoRa symbol at os samples per chip, carrying symbol id.
func upchirp(id, sf, os int) []complex128 {
	n := 1 << sf
	nSamp := n * os
	fold := nSamp - id*os
	out := make([]complex128, nSamp)
	nf := float64(n)
	osf := float64(os)
	for i := 0; i < nSamp; i++ {
		nn := float64(i)
		var ph float64
		if i < fold {
			ph = 2 * math.Pi * (nn*nn/(2*nf)/(osf*osf) + (float64(id)/nf-0.5)*nn/osf)
		} else {
			ph = 2 * math.Pi * (nn*nn/(2*nf)/(osf*osf) + (float64(id)/nf-1.5)*nn/osf)
		}
		out[i] = complex(math.Cos(ph), math.Sin(ph))
	}
	return out
}

// bestTiming is the sample offset that lines the preamble up with the
// reference chirp. -1 when nothing chirps hard enough to trust.
func bestTiming(x, down []complex128, sym int, buf []complex128) int {
	nSyms := len(x)/sym - 1
	if nSyms > 12 {
		nSyms = 12
	}
	if nSyms < 4 {
		return -1
	}
	step := sym / 32
	if step < 1 {
		step = 1
	}
	score := func(off int) float64 {
		sum := 0.0
		for s := 0; s < nSyms; s++ {
			_, r := symbolPeak(x, off+s*sym, down, buf)
			sum += r
		}
		return sum
	}
	best, bestScore := 0, -1.0
	try := func(off int) {
		if off < 0 || off >= sym {
			return
		}
		if sc := score(off); sc > bestScore {
			best, bestScore = off, sc
		}
	}
	for off := 0; off < sym; off += step {
		try(off)
	}
	mid := step / 4
	if mid < 1 {
		mid = 1
	}
	for off := best - step; off <= best+step; off += mid {
		try(off)
	}
	for off := best - mid; off <= best+mid; off++ {
		try(off)
	}
	if bestScore < float64(nSyms)*0.15 {
		return -1
	}
	return best
}

// findSFD returns the symbol index of the first downchirp after the preamble,
// and the preamble's FFT bin (the integer frequency offset). -1 if no
// downchirps are there.
func findSFD(x, up, down []complex128, sym, off int, buf []complex128) (int, int) {
	n := (len(x) - off) / sym
	if n > 48 {
		n = 48
	}
	type symInfo struct {
		upR, downR float64
		bin        int
	}
	syms := make([]symInfo, n)
	for s := 0; s < n; s++ {
		b, ur := symbolPeak(x, off+s*sym, down, buf)
		_, dr := symbolPeak(x, off+s*sym, up, buf)
		syms[s] = symInfo{ur, dr, b}
	}
	for s := 1; s < n-1; s++ {
		down1 := syms[s].downR > 0.2 && syms[s].downR > syms[s].upR*1.5
		down2 := syms[s+1].downR > 0.2 && syms[s+1].downR > syms[s+1].upR*1.5
		wasUp := syms[s-1].upR > 0.15 && syms[s-1].upR > syms[s-1].downR
		if down1 && down2 && wasUp {
			// The bin most of the upchirps share is the preamble. The two sync
			// symbols sit on different bins, so they lose the count.
			cnt := map[int]int{}
			cfo, nBin := 0, 0
			for _, p := range syms[:s] {
				if p.upR < 0.2 || p.upR < p.downR {
					continue
				}
				cnt[p.bin]++
				if cnt[p.bin] > nBin {
					cfo, nBin = p.bin, cnt[p.bin]
				}
			}
			return s, cfo
		}
	}
	return -1, 0
}

// headerAt demodulates the 8 header symbols starting at sample start and
// returns the coding rate when the header checksum matches.
func headerAt(x []complex128, sf, start, cfo int, down, buf []complex128) (int, bool) {
	nSym := 1 << sf
	sym := headerOS * nSym
	sfApp := sf - 2
	const cwLen = 8
	if start < 0 || start+cwLen*sym > len(x) {
		return 0, false
	}
	symbols := make([]int, cwLen)
	for i := 0; i < cwLen; i++ {
		bin, ratio := symbolPeak(x, start+i*sym, down, buf)
		if ratio < 0.08 {
			return 0, false
		}
		// Undo the receiver's frequency offset, the chirp's built-in +1, and
		// the two low bits the header never uses. What remains is gray coded.
		id := mod(bin-cfo, nSym)
		v := mod(id-1, nSym) / 4
		symbols[i] = v ^ (v >> 1)
	}
	cw := deinterleave(symbols, sfApp, cwLen)
	nib := make([]int, sfApp)
	for i := range nib {
		nib[i] = hammingNibble(cw[i])
	}
	return headerFields(nib)
}

// deinterleave undoes one LoRa block. symbols[i] bit j (MSB first) came from
// codeword (i-j-1) mod sfApp.
func deinterleave(symbols []int, sfApp, cwLen int) []int {
	cw := make([]int, sfApp)
	for i := 0; i < cwLen; i++ {
		for j := 0; j < sfApp; j++ {
			if (symbols[i]>>(sfApp-1-j))&1 == 0 {
				continue
			}
			row := mod(i-j-1, sfApp)
			cw[row] |= 1 << (cwLen - 1 - i)
		}
	}
	return cw
}

// hammingNibble decodes one coding-rate 4/8 codeword to its data nibble.
// The header is always sent at 4/8, whatever rate the payload uses.
func hammingNibble(cw int) int {
	bits := [8]int{}
	for i := 0; i < 8; i++ {
		bits[i] = (cw >> (7 - i)) & 1
	}
	data := [4]int{bits[3], bits[2], bits[1], bits[0]}
	ones := 0
	for _, b := range bits {
		ones += b
	}
	if ones%2 == 1 { // odd parity: a single bit may be wrong
		s0 := bits[0] ^ bits[1] ^ bits[2] ^ bits[4]
		s1 := bits[1] ^ bits[2] ^ bits[3] ^ bits[5]
		s2 := bits[0] ^ bits[1] ^ bits[3] ^ bits[6]
		switch s0 + s1<<1 + s2<<2 {
		case 5:
			data[3] ^= 1
		case 7:
			data[2] ^= 1
		case 3:
			data[1] ^= 1
		case 6:
			data[0] ^= 1
		}
	}
	return data[0]<<3 | data[1]<<2 | data[2]<<1 | data[3]
}

// headerFields reads length, coding rate, and checksum from the first five
// nibbles. ok is false when the checksum, length, or rate is impossible.
func headerFields(nib []int) (int, bool) {
	if len(nib) < 5 {
		return 0, false
	}
	payload := (nib[0] << 4) + nib[1]
	cr := nib[2] >> 1
	chk := ((nib[3] & 1) << 4) + nib[4]
	bit := func(v, i int) int { return (v >> i) & 1 }
	c4 := bit(nib[0], 3) ^ bit(nib[0], 2) ^ bit(nib[0], 1) ^ bit(nib[0], 0)
	c3 := bit(nib[0], 3) ^ bit(nib[1], 3) ^ bit(nib[1], 2) ^ bit(nib[1], 1) ^ bit(nib[2], 0)
	c2 := bit(nib[0], 2) ^ bit(nib[1], 3) ^ bit(nib[1], 0) ^ bit(nib[2], 3) ^ bit(nib[2], 1)
	c1 := bit(nib[0], 1) ^ bit(nib[1], 2) ^ bit(nib[1], 0) ^ bit(nib[2], 2) ^ bit(nib[2], 1) ^ bit(nib[2], 0)
	c0 := bit(nib[0], 0) ^ bit(nib[1], 1) ^ bit(nib[2], 3) ^ bit(nib[2], 2) ^ bit(nib[2], 1) ^ bit(nib[2], 0)
	sum := c4<<4 | c3<<3 | c2<<2 | c1<<1 | c0
	if chk != sum || payload == 0 || cr < 1 || cr > 4 {
		return 0, false
	}
	return cr, true
}

// symbolPeak multiplies one symbol by ref and returns the strongest FFT bin
// and the share of energy in it.
func symbolPeak(x []complex128, start int, ref, buf []complex128) (int, float64) {
	n := len(ref)
	if start < 0 || start+n > len(x) {
		return 0, 0
	}
	for i := 0; i < n; i++ {
		buf[i] = x[start+i] * ref[i]
	}
	fft(buf)
	total, peak, bin := 0.0, 0.0, 0
	for i, v := range buf {
		p := real(v)*real(v) + imag(v)*imag(v)
		total += p
		if p > peak {
			peak, bin = p, i
		}
	}
	if total == 0 {
		return 0, 0
	}
	return bin, peak / total
}

func mod(a, n int) int {
	r := a % n
	if r < 0 {
		r += n
	}
	return r
}

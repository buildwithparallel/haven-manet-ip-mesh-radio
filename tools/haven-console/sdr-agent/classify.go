package main

// Identify: listen to one frequency for a few seconds and say what each burst
// of signal is.
//
// 1. A spectrogram (512-point FFTs, 0.25 ms each) finds bursts: stretches of
//    time and frequency well above that frequency's usual noise.
// 2. Each narrow burst is tested for LoRa the way a LoRa receiver demodulates
//    it: multiply by a reference chirp ("dechirp"). With the right bandwidth
//    and spreading factor, every chirp turns into a single steady tone, so
//    the energy lands in one FFT bin; with the wrong ones it stays smeared.
//    The best bandwidth/SF pair, if clearly concentrated, is the answer.
//    The coding rate is not in the chirp; it's in the packet header, which is
//    read next (see lora_header.go).
// 3. A narrow burst that isn't LoRa is checked for FSK: its frequency jumps
//    between two values.
// 4. Bursts as wide as the whole view are wideband (OFDM, like Wi-Fi HaLow).

import (
	"encoding/json"
	"io"
	"math"
	"math/cmplx"
	"net/http"
	"sort"
	"time"
)

type burst struct {
	StartMs  float64 `json:"startMs"`
	DurMs    float64 `json:"durMs"`
	CenterHz float64 `json:"centerHz"`
	WidthHz  float64 `json:"widthHz"`
	SNR      float64 `json:"snr"`  // dB above that frequency's noise
	Kind     string  `json:"kind"` // lora, fsk, wide, tone, narrow
	BW       float64 `json:"bw,omitempty"`
	SF       int     `json:"sf,omitempty"`
	CR       int     `json:"cr,omitempty"`    // LoRa coding rate 1..4, meaning 4/5..4/8. 0 if the header didn't check
	Score    float64 `json:"score,omitempty"` // LoRa: share of energy in the dechirped tone
	DevHz    float64 `json:"devHz,omitempty"` // FSK: deviation
}

func (r *radio) identify(w http.ResponseWriter, req *http.Request) {
	center := num(req, "f", 915e6)
	secs := math.Max(1, math.Min(15, num(req, "secs", 6)))
	id, rd, err := r.take()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer r.done(id)
	r.gain(num(req, "gain", 0))
	r.tune(uint32(center))
	if catchUp(rd, r.rate) != nil || discard(rd, r.rate, 150*time.Millisecond) != nil {
		http.Error(w, "SDR stopped", http.StatusServiceUnavailable)
		return
	}
	raw := make([]byte, 2*int(secs*float64(r.rate)))
	if _, err := io.ReadFull(rd, raw); err != nil {
		http.Error(w, "SDR stopped", http.StatusServiceUnavailable)
		return
	}
	t := time.Now()
	bursts := analyze(raw, float64(r.rate), center)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{
		"center": center, "rate": r.rate, "secs": secs, "bursts": bursts,
		"halow": localHaLow(), "analysisMs": time.Since(t).Milliseconds(),
	})
}

// localHaLow reads this node's HaLow channel from its node agent:
// [center MHz, width MHz, primary width MHz, channel].
func localHaLow() []float64 {
	c := &http.Client{Timeout: 2 * time.Second}
	res, err := c.Get("http://127.0.0.1/cgi-bin/haven-node")
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	var any []map[string]json.RawMessage
	body, _ := io.ReadAll(res.Body)
	if json.Unmarshal(body, &any) != nil {
		var one map[string]json.RawMessage
		if json.Unmarshal(body, &one) != nil {
			return nil
		}
		any = append(any, one)
	}
	for _, m := range any {
		if h, ok := m["halow"]; ok {
			var v []float64
			if json.Unmarshal(h, &v) == nil && len(v) >= 2 {
				return v
			}
		}
	}
	return nil
}

func iqAt(raw []byte, k int) complex128 {
	return complex((float64(raw[2*k])-127.5)/127.5, (float64(raw[2*k+1])-127.5)/127.5)
}

func analyze(raw []byte, fs, center float64) []burst {
	const N = 512
	frameMs := float64(N) / fs * 1000
	frames := len(raw) / 2 / N
	if frames < 8 {
		return nil
	}
	// spectrogram in 0.5 dB steps (see toBytes)
	spec := make([]byte, frames*N)
	s := newSpectrum(N)
	pw := make([]float64, N)
	for f := 0; f < frames; f++ {
		for i := range pw {
			pw[i] = 0
		}
		s.power(raw[f*2*N:(f+1)*2*N], pw)
		toBytes(pw, spec[f*N:(f+1)*N])
	}
	// each bin's noise: its median over the capture
	floor := make([]int, N)
	for i := 0; i < N; i++ {
		var hist [256]int
		for f := 0; f < frames; f++ {
			hist[spec[f*N+i]]++
		}
		need, acc := frames/2, 0
		for v := 0; v < 256; v++ {
			acc += hist[v]
			if acc > need {
				floor[i] = v
				break
			}
		}
	}
	const hot = 20 // 10 dB above the noise
	// A bin counts only with a hot neighbor: noise makes lone hot bins, while
	// any real signal spreads across at least two.
	over := func(f, i int) bool { return i >= 0 && i < N && int(spec[f*N+i]) >= floor[i]+hot }
	isHot := func(f, i int) bool { return over(f, i) && (over(f, i-1) || over(f, i+1)) }
	// skip the bins at the very center: the RTL-SDR's own DC spike
	dc := func(i int) bool { return i >= N/2-2 && i <= N/2+2 }

	frameHot := make([]bool, frames)
	for f := 0; f < frames; f++ {
		n := 0
		for i := 0; i < N; i++ {
			if !dc(i) && isHot(f, i) {
				n++
			}
		}
		frameHot[f] = n >= 2
	}
	var out []burst
	for f := 0; f < frames; {
		if !frameHot[f] {
			f++
			continue
		}
		f0, f1, gap := f, f, 0
		for g := f + 1; g < frames && gap <= 8; g++ { // bridge gaps up to 2 ms
			if frameHot[g] {
				f1, gap = g, 0
			} else {
				gap++
			}
		}
		f = f1 + 1
		nf := f1 - f0 + 1
		count := make([]int, N)
		for g := f0; g <= f1; g++ {
			for i := 0; i < N; i++ {
				if !dc(i) && isHot(g, i) {
					count[i]++
				}
			}
		}
		need := int(math.Max(2, 0.03*float64(nf))) // low: a chirp crosses each bin only briefly
		// The bins at the tuned frequency are skipped (DC spike). A signal
		// that spans them must not be cut in two: fill the gap when both
		// sides are busy.
		if count[N/2-3] >= need && count[N/2+3] >= need {
			for i := N/2 - 2; i <= N/2+2; i++ {
				count[i] = need
			}
		}
		occ := 0
		for i := range count {
			if count[i] >= need {
				occ++
			}
		}
		// Wideband (OFDM like Wi-Fi HaLow): fills most of the view, or comes
		// as 3 or more separate pieces at once spread over 256 kHz or more.
		// (One LoRa or FSK transmission is a single block.)
		lo, hi, pieces := -1, -1, 0
		for i, last := 0, -100; i < N; i++ {
			if count[i] < need {
				continue
			}
			if lo < 0 {
				lo = i
			}
			hi = i
			if i-last > 4 {
				pieces++
			}
			last = i
		}
		scattered := pieces >= 3 && hi-lo >= N/8
		if occ > N*7/10 || scattered {
			// A very strong narrow signal (a radio right next to the SDR)
			// overloads the receiver and smears across the view. Its core,
			// everything within 10 dB of its peak, still shows its real
			// width: if that's narrow, test it like any other burst.
			if a, b, ok := strongCore(spec, f0, f1, N); ok && float64(b-a+1)*fs/N <= 700e3 {
				bu := burst{
					StartMs: float64(f0) * frameMs, DurMs: float64(nf) * frameMs,
					CenterHz: center + ((float64(a+b)/2)-float64(N/2))*fs/N,
					WidthHz:  float64(b-a+1) * fs / N,
					SNR:      peakSNR(spec, floor, f0, f1, a, b, N),
				}
				classify(&bu, raw, fs, center, f0*N, (f1+1)*N)
				if bu.Kind == "lora" {
					out = append(out, bu)
					continue
				}
			}
			out = append(out, burst{StartMs: float64(f0) * frameMs, DurMs: float64(nf) * frameMs, CenterHz: center + (float64(lo+hi)/2-float64(N/2))*fs/N, WidthHz: float64(hi-lo+1) * fs / N, SNR: peakSNR(spec, floor, f0, f1, 0, N-1, N), Kind: "wide"})
			continue
		}
		// contiguous runs of occupied bins are separate signals
		for i := 0; i < N; {
			if count[i] < need {
				i++
				continue
			}
			a, b, gap := i, i, 0
			for j := i + 1; j < N && gap <= 3; j++ {
				if count[j] >= need {
					b, gap = j, 0
				} else {
					gap++
				}
			}
			i = b + 1
			if nf < 2 {
				continue
			}
			bu := burst{
				StartMs: float64(f0) * frameMs, DurMs: float64(nf) * frameMs,
				CenterHz: center + ((float64(a+b)/2)-float64(N/2))*fs/N,
				WidthHz:  float64(b-a+1) * fs / N,
				SNR:      peakSNR(spec, floor, f0, f1, a, b, N),
			}
			classify(&bu, raw, fs, center, f0*N, (f1+1)*N)
			if bu.Kind == "tone" && bu.DurMs < 10 {
				continue // a blip this short and narrow is noise
			}
			out = append(out, bu)
		}
	}
	return out
}

// strongCore: the contiguous bins around a burst's peak that come within
// 10 dB of it (by each bin's maximum over the burst). ok is false when the
// burst has no clear peak.
func strongCore(spec []byte, f0, f1, N int) (int, int, bool) {
	mx := make([]int, N)
	top, at := 0, 0
	for i := 0; i < N; i++ {
		if i >= N/2-2 && i <= N/2+2 {
			continue // DC bins
		}
		for f := f0; f <= f1; f++ {
			if v := int(spec[f*N+i]); v > mx[i] {
				mx[i] = v
			}
		}
		if mx[i] > top {
			top, at = mx[i], i
		}
	}
	cut := top - 20 // 10 dB in 0.5 dB steps: LoRa is flat across its bandwidth, while clipping splatter sits ~15 dB down
	in := func(i int) bool { return i >= 0 && i < N && (mx[i] >= cut || (i >= N/2-2 && i <= N/2+2)) }
	a, b := at, at
	for in(a - 1) {
		a--
	}
	for in(b + 1) {
		b++
	}
	return a, b, b-a < N*9/10
}

func peakSNR(spec []byte, floor []int, f0, f1, a, b, N int) float64 {
	best := 0
	for f := f0; f <= f1; f++ {
		for i := a; i <= b; i++ {
			if d := int(spec[f*N+i]) - floor[i]; d > best {
				best = d
			}
		}
	}
	return float64(best) / 2
}

// classify tests one narrow burst (samples s0..s1) for LoRa, then FSK.
func classify(bu *burst, raw []byte, fs, center float64, s0, s1 int) {
	if bu.WidthHz < 12e3 {
		bu.Kind = "tone"
		return
	}
	bu.Kind = "narrow"
	if bu.WidthHz > 700e3 {
		bu.Kind = "wide"
		return
	}
	if max := int(0.6 * fs); s1-s0 > max { // the preamble is at the start; 0.6 s is plenty
		s1 = s0 + max
	}
	off := bu.CenterHz - center
	// Bandwidth first, from how far the frequency sweeps: settings with the
	// same chirp rate (like 250 kHz SF11 and 125 kHz SF9) dechirp equally
	// well, but sweep different widths. Then the spreading factor.
	bw := sweepWidth(raw, fs, off, s0, s1)
	best, bestBW, bestSF := 0.0, 0.0, 0
	x, fsd := downconvert(raw, fs, off, bw, s0, s1)
	for sf := 7; sf <= 12; sf++ {
		if sc := dechirpScore(x, fsd, bw, sf); sc > best {
			best, bestBW, bestSF = sc, bw, sf
		}
	}
	bu.Score = math.Round(best*100) / 100
	if best >= 0.3 {
		bu.Kind, bu.BW, bu.SF = "lora", bestBW, bestSF
		bu.CR = codingRate(raw, fs, off, bestBW, bestSF, s0, s1)
		return
	}
	if bu.DurMs < 8 {
		return // too short to tell FSK from a scrap of something wider
	}
	if dev, ok := fskDeviation(raw, fs, off, 2*math.Max(bu.WidthHz, 25e3), s0, s1); ok { // light filtering, so each bit's tone settles
		bu.Kind, bu.DevHz = "fsk", math.Round(dev)
	}
}

// sweepWidth: the span of the burst's instantaneous frequency (where the
// signal is strong), snapped to a LoRa bandwidth.
func sweepWidth(raw []byte, fs, off float64, s0, s1 int) float64 {
	x, fsd := downconvert(raw, fs, off, 500e3, s0, s1) // ±512 kHz: room for any LoRa bandwidth
	if len(x) < 64 {
		return 125e3
	}
	mag := make([]float64, len(x))
	for k, v := range x {
		mag[k] = real(v)*real(v) + imag(v)*imag(v)
	}
	m := append([]float64(nil), mag...)
	sort.Float64s(m)
	gate := m[len(m)/2] * 0.5
	var f []float64
	for k := 4; k < len(x); k++ {
		if mag[k] < gate || mag[k-4] < gate {
			continue
		}
		v := 0.0 // frequency, averaged over 4 one-sample steps (less noise, no wrap-around)
		for j := k - 3; j <= k; j++ {
			v += cmplx.Phase(x[j]*cmplx.Conj(x[j-1])) * fsd / (2 * math.Pi)
		}
		f = append(f, v/4)
	}
	if len(f) < 32 {
		return 125e3
	}
	sort.Float64s(f)
	span := f[len(f)*97/100] - f[len(f)*3/100]
	best, bd := 125e3, math.Inf(1)
	for _, bw := range []float64{62.5e3, 125e3, 250e3, 500e3} {
		if d := math.Abs(math.Log(span / bw)); d < bd {
			best, bd = bw, d
		}
	}
	return best
}

// downconvert shifts the burst to 0 Hz, then sums blocks of D samples (a
// simple low-pass) to a rate of about twice bw.
func downconvert(raw []byte, fs, off, bw float64, s0, s1 int) ([]complex128, float64) {
	D := int(math.Max(1, math.Floor(fs/(2*bw))))
	var mean complex128
	for k := s0; k < s1; k++ {
		mean += iqAt(raw, k)
	}
	mean /= complex(float64(s1-s0), 0)
	out := make([]complex128, 0, (s1-s0)/D+1)
	step := -2 * math.Pi * off / fs
	var acc complex128
	for k := s0; k < s1; k++ {
		ph := step * float64(k-s0)
		acc += (iqAt(raw, k) - mean) * complex(math.Cos(ph), math.Sin(ph))
		if (k-s0)%D == D-1 {
			out = append(out, acc)
			acc = 0
		}
	}
	return out, fs / float64(D)
}

// dechirpScore: average share of energy in the strongest bin (and its
// neighbors) after dechirping one symbol at a time.
func dechirpScore(x []complex128, fsd, bw float64, sf int) float64 {
	ts := math.Pow(2, float64(sf)) / bw
	ns := int(math.Round(ts * fsd))
	if ns < 16 || len(x) < 2*ns {
		return 0
	}
	ref := make([]complex128, ns)
	for k := range ref {
		t := float64(k) / fsd
		ph := 2 * math.Pi * (-bw/2*t + bw/(2*ts)*t*t)
		ref[k] = complex(math.Cos(ph), -math.Sin(ph)) // conjugate upchirp
	}
	n := 1
	for n < ns {
		n <<= 1
	}
	buf := make([]complex128, n)
	sum, used := 0.0, 0
	for w := 0; w+ns <= len(x) && used < 12; w += ns {
		for k := range buf {
			buf[k] = 0
		}
		for k := 0; k < ns; k++ {
			buf[k] = x[w+k] * ref[k]
		}
		fft(buf)
		total, peak, pi := 0.0, 0.0, 0
		for k, v := range buf {
			p := real(v)*real(v) + imag(v)*imag(v)
			total += p
			if p > peak {
				peak, pi = p, k
			}
		}
		if total == 0 {
			continue
		}
		e := 0.0
		for d := -2; d <= 2; d++ {
			v := buf[(pi+d+n)%n]
			e += real(v)*real(v) + imag(v)*imag(v)
		}
		sum += e / total
		used++
	}
	if used == 0 {
		return 0
	}
	return sum / float64(used)
}

// fskDeviation: does the burst's frequency sit at two values? Returns half
// the distance between them.
func fskDeviation(raw []byte, fs, off, width float64, s0, s1 int) (float64, bool) {
	x, fsd := downconvert(raw, fs, off, width, s0, s1)
	if len(x) < 64 {
		return 0, false
	}
	f := make([]float64, 0, len(x))
	for k := 1; k < len(x); k++ {
		f = append(f, cmplx.Phase(x[k]*cmplx.Conj(x[k-1]))*fsd/(2*math.Pi))
	}
	sort.Float64s(f)
	// best split into two groups (Otsu)
	n := len(f)
	pre := make([]float64, n+1)
	for i, v := range f {
		pre[i+1] = pre[i] + v
	}
	bestVar, cut := -1.0, 0
	for i := n / 10; i < n*9/10; i++ {
		m1, m2 := pre[i]/float64(i), (pre[n]-pre[i])/float64(n-i)
		v := float64(i) * float64(n-i) * (m1 - m2) * (m1 - m2)
		if v > bestVar {
			bestVar, cut = v, i
		}
	}
	if cut == 0 {
		return 0, false
	}
	m1, m2 := pre[cut]/float64(cut), (pre[n]-pre[cut])/float64(n-cut)
	// spread of each group: median distance from its middle (ignores the
	// in-between values while the frequency moves from one tone to the other)
	spread := func(a, b int, _ float64) float64 {
		mid := f[(a+b)/2]
		d := make([]float64, 0, b-a)
		for _, v := range f[a:b] {
			d = append(d, math.Abs(v-mid))
		}
		sort.Float64s(d)
		return 1.48 * d[len(d)/2]
	}
	sep := m2 - m1
	if sep > 4e3 && sep > 2.5*(spread(0, cut, m1)+spread(cut, n, m2))/2 {
		return sep / 2, true
	}
	return 0, false
}

// fft is an in-place radix-2 FFT; len(a) must be a power of two.
func fft(a []complex128) {
	n := len(a)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		w := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			wk := complex(1, 0)
			for k := 0; k < size/2; k++ {
				u, v := a[start+k], a[start+k+size/2]*wk
				a[start+k], a[start+k+size/2] = u+v, u-v
				wk *= w
			}
		}
	}
}

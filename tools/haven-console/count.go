package main

// Count track: when switched on from the Talk tab, the gate joins the channel
// like a phone and loops a counting recording to everyone on it. Listeners on
// other nodes hear it across HaLow, which makes reroutes audible.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

type signalData struct {
	SDP       *webrtc.SessionDescription `json:"sdp,omitempty"`
	Candidate *webrtc.ICECandidateInit   `json:"candidate,omitempty"`
}

type countSource struct {
	hub   *hub
	name  string
	track []byte // Ogg Opus with 20 ms pages
	api   *webrtc.API

	mu    sync.Mutex
	me    *talker
	stop  chan struct{}
	peers map[string]*webrtc.PeerConnection
	cands map[string][]webrtc.ICECandidateInit
	out   *webrtc.TrackLocalStaticSample
}

func newCountSource(h *hub, name string, track []byte) *countSource {
	se := webrtc.SettingEngine{}
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeQueryOnly)
	return &countSource{hub: h, name: name, track: track, api: webrtc.NewAPI(webrtc.WithSettingEngine(se))}
}

func (c *countSource) on() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.me != nil
}

func (c *countSource) start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.me != nil {
		return nil
	}
	out, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio", "haven-count")
	if err != nil {
		return err
	}
	b := make([]byte, 12)
	rand.Read(b)
	c.me = &talker{id: "count-" + hex.EncodeToString(b), name: c.name, ch: make(chan []byte, 256)}
	c.stop = make(chan struct{})
	c.peers = map[string]*webrtc.PeerConnection{}
	c.cands = map[string][]webrtc.ICECandidateInit{}
	c.out = out

	others, _ := c.hub.join(c.me)
	go c.loop(c.me, c.stop)
	go c.play(out, c.stop)
	for _, p := range others { // a newcomer calls everyone already there
		go c.offer(p["id"], false)
	}
	log.Printf("count: on, calling %d peer(s)", len(others))
	return nil
}

func (c *countSource) halt() {
	c.mu.Lock()
	me, stop, peers := c.me, c.stop, c.peers
	c.me, c.stop, c.peers, c.cands, c.out = nil, nil, nil, nil, nil
	c.mu.Unlock()
	if me == nil {
		return
	}
	close(stop)
	c.hub.leave(me)
	for _, pc := range peers {
		pc.Close()
	}
	log.Printf("count: off")
}

func (c *countSource) post(to string, d signalData) {
	c.mu.Lock()
	me := c.me
	c.mu.Unlock()
	if me == nil {
		return
	}
	raw, _ := json.Marshal(d)
	c.hub.send(to, map[string]any{"type": "signal", "from": me.id, "data": json.RawMessage(raw)})
}

func (c *countSource) peer(id string) (*webrtc.PeerConnection, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.me == nil {
		return nil, errors.New("count is off")
	}
	if pc := c.peers[id]; pc != nil {
		return pc, nil
	}
	pc, err := c.api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}
	sender, err := pc.AddTrack(c.out)
	if err != nil {
		pc.Close()
		return nil, err
	}
	go func() { // drain RTCP so the sender keeps working
		buf := make([]byte, 1500)
		for {
			if _, _, err := sender.Read(buf); err != nil {
				return
			}
		}
	}()
	myID := c.me.id
	pc.OnICECandidate(func(cand *webrtc.ICECandidate) {
		if cand != nil {
			init := cand.ToJSON()
			c.post(id, signalData{Candidate: &init})
		}
	})
	pc.OnConnectionStateChange(func(st webrtc.PeerConnectionState) {
		log.Printf("count: %s %s", id, st)
		if st == webrtc.PeerConnectionStateFailed && myID < id {
			go c.offer(id, true)
		}
	})
	c.peers[id] = pc
	return pc, nil
}

func (c *countSource) offer(id string, restart bool) {
	pc, err := c.peer(id)
	if err != nil {
		return
	}
	o, err := pc.CreateOffer(&webrtc.OfferOptions{ICERestart: restart})
	if err == nil {
		err = pc.SetLocalDescription(o)
	}
	if err != nil {
		log.Printf("count: offer to %s: %v", id, err)
		return
	}
	c.post(id, signalData{SDP: pc.LocalDescription()})
}

func (c *countSource) onSignal(from string, raw json.RawMessage) {
	var d signalData
	if json.Unmarshal(raw, &d) != nil {
		return
	}
	pc, err := c.peer(from)
	if err != nil {
		return
	}
	if d.SDP != nil {
		if err := pc.SetRemoteDescription(*d.SDP); err != nil {
			log.Printf("count: remote description from %s: %v", from, err)
			return
		}
		c.mu.Lock()
		pending := c.cands[from]
		delete(c.cands, from)
		c.mu.Unlock()
		for _, cand := range pending {
			pc.AddICECandidate(cand)
		}
		if d.SDP.Type == webrtc.SDPTypeOffer {
			a, err := pc.CreateAnswer(nil)
			if err == nil {
				err = pc.SetLocalDescription(a)
			}
			if err != nil {
				log.Printf("count: answer to %s: %v", from, err)
				return
			}
			c.post(from, signalData{SDP: pc.LocalDescription()})
		}
	} else if d.Candidate != nil {
		if pc.RemoteDescription() == nil {
			c.mu.Lock()
			if c.cands != nil {
				c.cands[from] = append(c.cands[from], *d.Candidate)
			}
			c.mu.Unlock()
		} else {
			pc.AddICECandidate(*d.Candidate)
		}
	}
}

// loop handles hub messages addressed to the count peer.
func (c *countSource) loop(me *talker, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case b, ok := <-me.ch:
			if !ok {
				return
			}
			var ev struct {
				Type string          `json:"type"`
				ID   string          `json:"id"`
				From string          `json:"from"`
				Data json.RawMessage `json:"data"`
			}
			if json.Unmarshal(b, &ev) != nil {
				continue
			}
			switch ev.Type {
			case "signal":
				go c.onSignal(ev.From, ev.Data)
			case "leave":
				c.mu.Lock()
				pc := c.peers[ev.ID]
				delete(c.peers, ev.ID)
				c.mu.Unlock()
				if pc != nil {
					pc.Close()
				}
			}
		}
	}
}

// play loops the track into the outgoing audio in real time.
func (c *countSource) play(out *webrtc.TrackLocalStaticSample, stop chan struct{}) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		ogg, _, err := oggreader.NewWith(bytes.NewReader(c.track))
		if err != nil {
			log.Printf("count: track: %v", err)
			return
		}
		var last uint64
		for {
			page, hdr, err := ogg.ParseNextPage()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				log.Printf("count: ogg: %v", err)
				break
			}
			samples := hdr.GranulePosition - last
			last = hdr.GranulePosition
			if samples == 0 || samples > 48000 {
				continue // header pages and the stream start
			}
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
			out.WriteSample(media.Sample{Data: page, Duration: time.Duration(samples) * time.Second / 48000})
		}
	}
}

func (c *countSource) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			On bool `json:"on"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&req) != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.On {
			if err := c.start(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			c.halt()
		}
	}
	writeJSON(w, map[string]any{"on": c.on(), "name": c.name})
}

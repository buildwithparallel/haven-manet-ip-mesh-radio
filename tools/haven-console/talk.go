package main

// Talk: the meeting point where browsers find each other for WebRTC calls.
// Audio never passes through here; it goes phone to phone over the mesh.

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

var validID = regexp.MustCompile(`^[A-Za-z0-9-]{4,64}$`)

type talker struct {
	id, name string
	ch       chan []byte
}

type hub struct {
	mu    sync.Mutex
	peers map[string]*talker
}

func newHub() *hub { return &hub{peers: map[string]*talker{}} }

func encode(msg any) []byte {
	b, _ := json.Marshal(msg)
	return b
}

// sendLocked queues a message without blocking. Caller holds h.mu.
func (h *hub) sendLocked(p *talker, b []byte) {
	select {
	case p.ch <- b:
	default:
		log.Printf("talk: dropping message for slow peer %s", p.name)
	}
}

func (h *hub) send(id string, msg any) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.peers[id]
	if ok {
		h.sendLocked(p, encode(msg))
	}
	return ok
}

func (h *hub) broadcastLocked(except string, msg any) {
	b := encode(msg)
	for id, p := range h.peers {
		if id != except {
			h.sendLocked(p, b)
		}
	}
}

// join registers a peer and returns the others already on the channel. If the
// id is already registered (a browser reconnecting), the old stream is closed.
func (h *hub) join(p *talker) (others []map[string]string, rejoin bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	old := h.peers[p.id]
	h.peers[p.id] = p
	for id, q := range h.peers {
		if id != p.id {
			others = append(others, map[string]string{"id": id, "name": q.name})
		}
	}
	if old != nil {
		close(old.ch)
		return others, true
	}
	h.broadcastLocked(p.id, map[string]string{"type": "join", "id": p.id, "name": p.name})
	return others, false
}

func (h *hub) leave(p *talker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.peers[p.id] == p {
		delete(h.peers, p.id)
		h.broadcastLocked(p.id, map[string]string{"type": "leave", "id": p.id})
	}
}

func (h *hub) events(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if !validID.MatchString(id) {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if name == "" {
		name = "guest"
	}
	if len(name) > 32 {
		name = name[:32]
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	p := &talker{id: id, name: name, ch: make(chan []byte, 256)}
	others, rejoin := h.join(p)
	if !rejoin {
		log.Printf("talk: %s joined from %s", name, r.RemoteAddr)
	}
	w.Write([]byte("data: " + string(encode(map[string]any{"type": "welcome", "id": id, "peers": others})) + "\n\n"))
	flusher.Flush()

	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case b, ok := <-p.ch:
			if !ok {
				return // replaced by a newer stream from the same browser
			}
			if _, err := w.Write([]byte("data: " + string(b) + "\n\n")); err != nil {
				h.leave(p)
				return
			}
			flusher.Flush()
		case <-ping.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				h.leave(p)
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			h.leave(p)
			log.Printf("talk: %s left", name)
			return
		}
	}
}

func (h *hub) signal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var m struct {
		From string          `json:"from"`
		To   string          `json:"to"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&m); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if !validID.MatchString(m.From) || !validID.MatchString(m.To) {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if !h.send(m.To, map[string]any{"type": "signal", "from": m.From, "data": m.Data}) {
		http.Error(w, "peer not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Package web serves a page showing the cluster state in the browser, like
// cluster-smi does in the terminal, and streams the state to it.
//
// The package does not depend on where the state comes from: whoever has it
// passes it to Server.Update. cluster-smi-router passes its own state; a
// standalone server could pass what routerclient.Poll receives. The server
// does not process the state either: sorting, filtering and formatting
// happen in the browser.
package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/mehu-ukko/cluster-smi/cluster"
)

//go:embed static
var static embed.FS

// keepAlive is the interval of comments sent on idle event streams, so
// proxies do not close them.
const keepAlive = 30 * time.Second

// state is the message sent to the browser.
type state struct {
	Timeout int            `json:"timeout"` // seconds after which a node is considered offline
	Nodes   []cluster.Node `json:"nodes"`
}

// Server keeps the latest cluster state and streams it to browsers.
type Server struct {
	timeout int

	mu     sync.Mutex
	latest []byte                   // JSON of the latest state, nil before the first update
	subs   map[chan []byte]struct{} // open event streams
}

// New returns a Server reporting nodes as offline after timeout seconds.
func New(timeout int) *Server {
	return &Server{timeout: timeout, subs: make(map[chan []byte]struct{})}
}

// Update stores c as the latest state and sends it to all open event streams.
func (s *Server) Update(c cluster.Cluster) error {
	msg, err := json.Marshal(state{Timeout: s.timeout, Nodes: c.Nodes})
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest = msg
	for ch := range s.subs {
		// drop the pending state of a slow browser, only the latest matters
		select {
		case <-ch:
		default:
		}
		ch <- msg
	}
	return nil
}

// Handler returns the HTTP handler serving the page, the latest state at
// /api/cluster and a stream of states at /api/events.
func (s *Server) Handler() http.Handler {
	files, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(files))
	mux.HandleFunc("GET /api/cluster", s.serveState)
	mux.HandleFunc("GET /api/events", s.serveEvents)
	return mux
}

func (s *Server) subscribe() (chan []byte, []byte) {
	ch := make(chan []byte, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[ch] = struct{}{}
	return ch, s.latest
}

func (s *Server) unsubscribe(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, ch)
}

func (s *Server) serveState(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	msg := s.latest
	s.mu.Unlock()

	if msg == nil {
		http.Error(w, "no cluster state yet", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(msg)
}

// serveEvents streams the states as server-sent events.
func (s *Server) serveEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)

	ch, latest := s.subscribe()
	defer s.unsubscribe(ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no") // disable buffering in nginx

	send := func(msg []byte) bool {
		if _, err := w.Write([]byte("data: " + string(msg) + "\n\n")); err != nil {
			return false
		}
		return rc.Flush() == nil
	}

	// start with the latest state, so the page does not wait for the next tick
	if latest != nil {
		if !send(latest) {
			return
		}
	} else {
		w.WriteHeader(http.StatusOK)
		if rc.Flush() != nil {
			return
		}
	}

	ticker := time.NewTicker(keepAlive)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			if !send(msg) {
				return
			}
		case <-ticker.C:
			if _, err := w.Write([]byte(": keep-alive\n\n")); err != nil || rc.Flush() != nil {
				return
			}
		}
	}
}

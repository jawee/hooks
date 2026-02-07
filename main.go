package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RequestInfo struct {
	Timestamp string
	Headers   map[string][]string
	Body      string
}

type ListenerStore struct {
	sync.RWMutex
	listeners map[string][]RequestInfo               // uuid -> list of requests
	conns     map[string]map[*websocketConn]struct{} // uuid -> set of websocket connections
}

type websocketConn struct {
	send  chan string
	close chan struct{}
	w     http.ResponseWriter
	r     *http.Request
}

var (
	templates = template.Must(template.ParseFiles("templates/index.html", "templates/listener.html"))
	store     = &ListenerStore{listeners: make(map[string][]RequestInfo), conns: make(map[string]map[*websocketConn]struct{})}
)

func main() {
	// For development: create a default listener
	devUUID := "800e7e855f2230ceb5d78edb66c87f63"
	store.Lock()
	if _, exists := store.listeners[devUUID]; !exists {
		store.listeners[devUUID] = []RequestInfo{}
	}
	store.Unlock()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/create-listener", createListenerHandler)
	http.HandleFunc("/listener/", listenerHandler)
	http.HandleFunc("/ws/", wsHandler)
	log.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	store.RLock()
	var uuids []string
	for uuid := range store.listeners {
		uuids = append(uuids, uuid)
	}
	store.RUnlock()
	templates.ExecuteTemplate(w, "index.html", map[string]interface{}{
		"Listeners": uuids,
	})
}

func createListenerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	uuid := newUUID()
	store.Lock()
	store.listeners[uuid] = []RequestInfo{}
	store.Unlock()
	http.Redirect(w, r, "/listener/"+uuid, http.StatusSeeOther)
}

func listenerHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Path[len("/listener/"):]
	if uuid == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		store.RLock()
		reqs, ok := store.listeners[uuid]
		store.RUnlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		templates.ExecuteTemplate(w, "listener.html", map[string]interface{}{
			"UUID":     uuid,
			"Requests": reqs,
		})
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		reqInfo := RequestInfo{
			Timestamp: time.Now().Format(time.RFC3339),
			Headers:   r.Header,
			Body:      string(body),
		}
		store.Lock()
		if _, ok := store.listeners[uuid]; ok {
			store.listeners[uuid] = append(store.listeners[uuid], reqInfo)
			// Broadcast to websockets as JSON
			msgBytes, _ := json.Marshal(reqInfo)
			for conn := range store.conns[uuid] {
				select {
				case conn.send <- string(msgBytes):
				default:
				}
			}
		}
		store.Unlock()
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func headersToString(h map[string][]string) string {
	var sb strings.Builder
	for k, v := range h {
		sb.WriteString(k + ": " + strings.Join(v, ", ") + "\n")
	}
	return sb.String()
}

// --- WebSocket handler (simple, not RFC6455 compliant, for demo only) ---
func wsHandler(w http.ResponseWriter, r *http.Request) {
	uuid := strings.TrimPrefix(r.URL.Path, "/ws/")
	if uuid == "" {
		http.NotFound(w, r)
		return
	}
	// Upgrade to websocket (very basic, for demo; for production use gorilla/websocket)
	if r.Header.Get("Connection") != "Upgrade" || r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "Not a websocket handshake", http.StatusBadRequest)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "hijack failed", http.StatusInternalServerError)
		return
	}
	// Write minimal handshake
	key := r.Header.Get("Sec-WebSocket-Key")
	accept := computeAcceptKey(key)
	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: websocket\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n")
	bufrw.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
	bufrw.Flush()
	ws := &websocketConn{send: make(chan string, 10), close: make(chan struct{}), w: w, r: r}
	store.Lock()
	if store.conns[uuid] == nil {
		store.conns[uuid] = make(map[*websocketConn]struct{})
	}
	store.conns[uuid][ws] = struct{}{}
	store.Unlock()
	go wsWriter(conn, ws, uuid)
	// No reader: this is a one-way push
}

func wsWriter(conn net.Conn, ws *websocketConn, uuid string) {
	defer func() {
		conn.Close()
		store.Lock()
		delete(store.conns[uuid], ws)
		store.Unlock()
	}()
	for {
		select {
		case msg := <-ws.send:
			// Write a text frame (very basic, not handling fragmentation, masking, etc.)
			frame := []byte{0x81}
			l := len(msg)
			if l < 126 {
				frame = append(frame, byte(l))
			} else if l < 65536 {
				frame = append(frame, 126, byte(l>>8), byte(l))
			} else {
				frame = append(frame, 127, 0, 0, 0, 0, byte(l>>24), byte(l>>16), byte(l>>8), byte(l))
			}
			frame = append(frame, []byte(msg)...)
			conn.Write(frame)
		case <-ws.close:
			return
		}
	}
}

// Compute Sec-WebSocket-Accept (RFC6455)
func computeAcceptKey(key string) string {
	const magic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1Sum(key + magic)
	return base64Encode(h)
}

func sha1Sum(s string) []byte {
	h := sha1.New()
	h.Write([]byte(s))
	return h.Sum(nil)
}

func base64Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func newUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

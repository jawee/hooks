package main

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"webhooktester/templates"
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
	// user -> uuid -> []RequestInfo
	userListeners = struct {
		sync.RWMutex
		data map[string]map[string][]RequestInfo
	}{data: make(map[string]map[string][]RequestInfo)}
	// username -> password (hardcoded for demo, but can register new users)
	users = struct {
		sync.RWMutex
		data map[string]string
	}{data: map[string]string{"demo": "demo123"}}
	// sessionID -> username
	sessions = struct {
		sync.RWMutex
		data map[string]string
	}{data: make(map[string]string)}
	// For websocket conns: user -> uuid -> set of conns
	wsConns = struct {
		sync.RWMutex
		data map[string]map[string]map[*websocketConn]struct{}
	}{data: make(map[string]map[string]map[*websocketConn]struct{})}
)

func main() {
	// Ensure demo user has a listener map
	userListeners.Lock()
	if userListeners.data["demo"] == nil {
		userListeners.data["demo"] = make(map[string][]RequestInfo)
	}
	userListeners.Unlock()
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/", withAuth(indexHandler))
	http.HandleFunc("/create-listener", withAuth(createListenerHandler))
	// Split /listener/ routing: GETs require auth, POSTs are anonymous
	http.HandleFunc("/listener/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			withAuth(listenerHandler)(w, r)
		} else {
			listenerHandler(w, r)
		}
	})
	http.HandleFunc("/ws/", withAuth(wsHandler))
	log.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// --- Auth Middleware and Helpers ---
func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := getUsernameFromSession(r)
		log.Printf("[DEBUG] withAuth: session_id=%v username=%v", r.Header.Get("Cookie"), username)
		if username == "" {
			log.Printf("[DEBUG] withAuth: No username in session, redirecting to login")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		// Attach username to context
		ctx := r.Context()
		ctx = contextWithUsername(ctx, username)
		next(w, r.WithContext(ctx))
	}
}

func getUsernameFromSession(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}
	sessions.RLock()
	defer sessions.RUnlock()
	return sessions.data[cookie.Value]
}

type contextKey string

const usernameKey = contextKey("username")

func contextWithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, usernameKey, username)
}
func getUsername(r *http.Request) string {
	v := r.Context().Value(usernameKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// --- Registration/Login/Logout Handlers ---
func registerHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodGet {
		templates.Register("").Render(r.Context(), w)
		return
	}
	if err := r.ParseForm(); err != nil {
		templates.Register("Invalid form").Render(r.Context(), w)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		templates.Register("Username and password required").Render(r.Context(), w)
		return
	}
	users.Lock()
	if _, exists := users.data[username]; exists {
		users.Unlock()
		templates.Register("Username already exists").Render(r.Context(), w)
		return
	}
	users.data[username] = password
	users.Unlock()
	// Create empty listener map for user
	userListeners.Lock()
	userListeners.data[username] = make(map[string][]RequestInfo)
	userListeners.Unlock()
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		templates.Login("").Render(r.Context(), w)
		return
	}
	if err := r.ParseForm(); err != nil {
		templates.Login("Invalid form").Render(r.Context(), w)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	users.RLock()
	realpw, exists := users.data[username]
	users.RUnlock()
	if !exists || realpw != password {
		templates.Login("Invalid credentials").Render(r.Context(), w)
		return
	}
	// Set session
	sessionID := newUUID()
	sessions.Lock()
	sessions.data[sessionID] = username
	sessions.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "session_id", Value: sessionID, Path: "/", HttpOnly: true, MaxAge: 3600})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {

	cookie, err := r.Cookie("session_id")
	if err == nil {
		sessions.Lock()
		delete(sessions.data, cookie.Value)
		sessions.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "", Path: "/", MaxAge: -1})
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
func indexHandler(w http.ResponseWriter, r *http.Request) {
	username := getUsername(r)
	userListeners.RLock()
	var uuids []string
	for uuid := range userListeners.data[username] {
		uuids = append(uuids, uuid)
	}
	userListeners.RUnlock()
	templates.Index(username, uuids).Render(r.Context(), w)
}

func createListenerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	username := getUsername(r)
	log.Printf("[DEBUG] createListenerHandler: username=%v", username)
	uuid := newUUID()
	userListeners.Lock()
	if userListeners.data[username] == nil {
		log.Printf("[DEBUG] createListenerHandler: userListeners.data[%v] is nil, initializing", username)
		userListeners.data[username] = make(map[string][]RequestInfo)
	}
	userListeners.data[username][uuid] = []RequestInfo{}
	userListeners.Unlock()
	log.Printf("[DEBUG] createListenerHandler: created uuid=%v for user=%v", uuid, username)
	http.Redirect(w, r, "/listener/"+uuid, http.StatusSeeOther)
}

func listenerHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Path[len("/listener/"):]
	if uuid == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		// Anonymous POST: find which user owns this uuid
		userListeners.Lock()
		var owner string
		for user, m := range userListeners.data {
			if _, ok := m[uuid]; ok {
				owner = user
				break
			}
		}
		if owner == "" {
			userListeners.Unlock()
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			userListeners.Unlock()
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		reqInfo := RequestInfo{
			Timestamp: time.Now().Format(time.RFC3339),
			Headers:   r.Header,
			Body:      string(body),
		}
		userListeners.data[owner][uuid] = append(userListeners.data[owner][uuid], reqInfo)
		// Broadcast to websockets as JSON
		wsConns.RLock()
		for conn := range wsConns.data[owner][uuid] {
			select {
			case conn.send <- string(mustJSON(reqInfo)):
			default:
			}
		}
		wsConns.RUnlock()
		userListeners.Unlock()
		w.WriteHeader(http.StatusOK)
		return
	}
	// Auth required for GET
	username := getUsername(r)
	log.Printf("[DEBUG] listenerHandler GET: username=%v uuid=%v", username, uuid)
	if username == "" {
		log.Printf("[DEBUG] listenerHandler GET: No username in context, redirecting to login")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	switch r.Method {
	case http.MethodGet:
		userListeners.RLock()
		reqs, ok := userListeners.data[username][uuid]
		userListeners.RUnlock()
		log.Printf("[DEBUG] listenerHandler GET: userListeners.data[%v][%v] exists=%v", username, uuid, ok)
		if !ok {
			http.NotFound(w, r)
			return
		}
		// Convert []RequestInfo to []templates.RequestInfo
		templReqs := make([]templates.RequestInfo, len(reqs))
		for i, req := range reqs {
			templReqs[i] = templates.RequestInfo{
				Timestamp: req.Timestamp,
				Headers:   req.Headers,
				Body:      req.Body,
			}
		}
		templates.Listener(uuid, templReqs).Render(r.Context(), w)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
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
	username := getUsername(r)
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
	wsConns.Lock()
	if wsConns.data[username] == nil {
		wsConns.data[username] = make(map[string]map[*websocketConn]struct{})
	}
	if wsConns.data[username][uuid] == nil {
		wsConns.data[username][uuid] = make(map[*websocketConn]struct{})
	}
	wsConns.data[username][uuid][ws] = struct{}{}
	wsConns.Unlock()
	go wsWriter(conn, ws, username, uuid)
	// No reader: this is a one-way push
}

func wsWriter(conn net.Conn, ws *websocketConn, username, uuid string) {
	defer func() {
		conn.Close()
		wsConns.Lock()
		delete(wsConns.data[username][uuid], ws)
		wsConns.Unlock()
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

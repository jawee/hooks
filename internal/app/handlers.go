package app

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

dbsqlc "webhooktester/db/sqlc"
"webhooktester/templates"
)

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

func (a *App) registerHandler(w http.ResponseWriter, r *http.Request) {
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
	_, err := a.Queries.GetUserByUsername(r.Context(), username)
	if err == nil {
		templates.Register("Username already exists").Render(r.Context(), w)
		return
	}
	if err != sql.ErrNoRows {
		templates.Register("Internal error").Render(r.Context(), w)
		return
	}
	_, err = a.Queries.CreateUser(r.Context(), dbsqlc.CreateUserParams{
		Username:     username,
		PasswordHash: password,
	})
	if err != nil {
		templates.Register("Failed to create user").Render(r.Context(), w)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) loginHandler(w http.ResponseWriter, r *http.Request) {
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
	user, err := a.Queries.GetUserByUsername(r.Context(), username)
	if err != nil || user.PasswordHash != password {
		templates.Login("Invalid credentials").Render(r.Context(), w)
		return
	}
	sessionID := newUUID()
	_, err = a.Queries.CreateSession(r.Context(), dbsqlc.CreateSessionParams{
		SessionID: sessionID,
		UserID:    user.ID,
	})
	if err != nil {
		templates.Login("Internal error").Render(r.Context(), w)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session_id", Value: sessionID, Path: "/", HttpOnly: true, MaxAge: 3600})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		_ = a.Queries.DeleteSession(r.Context(), cookie.Value)
		http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "", Path: "/", MaxAge: -1})
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) indexHandler(w http.ResponseWriter, r *http.Request) {
	username := getUsername(r)
	user, err := a.Queries.GetUserByUsername(r.Context(), username)
	if err != nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}
	listeners, err := a.Queries.GetListenersByUser(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "Failed to fetch listeners", http.StatusInternalServerError)
		return
	}
	var uuids []string
	for _, l := range listeners {
		uuids = append(uuids, l.Uuid)
	}
	templates.Index(username, uuids).Render(r.Context(), w)
}

func (a *App) createListenerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	username := getUsername(r)
	log.Printf("[DEBUG] createListenerHandler: username=%v", username)
	user, err := a.Queries.GetUserByUsername(r.Context(), username)
	if err != nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}
	uuid := newUUID()
	_, err = a.Queries.CreateListener(r.Context(), dbsqlc.CreateListenerParams{
		Uuid:   uuid,
		UserID: user.ID,
	})
	if err != nil {
		http.Error(w, "Failed to create listener", http.StatusInternalServerError)
		return
	}
	log.Printf("[DEBUG] createListenerHandler: created uuid=%v for user=%v", uuid, username)
	http.Redirect(w, r, "/listener/"+uuid, http.StatusSeeOther)
}

func (a *App) listenerHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Path[len("/listener/") :]
	if uuid == "" {
		http.NotFound(w, r)
		return
	}
	listener, err := a.Queries.GetListenerByUUID(r.Context(), uuid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		headers, err := json.Marshal(r.Header)
		if err != nil {
			http.Error(w, "Failed to encode headers", http.StatusInternalServerError)
			return
		}
		_, err = a.Queries.CreateRequest(r.Context(), dbsqlc.CreateRequestParams{
			ListenerID: listener.ID,
			Headers:    headers,
			Body:       string(body),
		})
		if err != nil {
			http.Error(w, "Failed to save request", http.StatusInternalServerError)
			return
		}
		// TODO: Broadcast to websockets for real-time update (move wsConns logic here if needed)
		w.WriteHeader(http.StatusOK)
		return
	}
	// Auth required for GET
	username := getUsername(r)
	if username == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := a.Queries.GetUserByUsername(r.Context(), username)
	if err != nil || user.ID != listener.UserID {
		http.NotFound(w, r)
		return
	}
	reqs, err := a.Queries.GetRequestsByListener(r.Context(), listener.ID)
	if err != nil {
		http.Error(w, "Failed to fetch requests", http.StatusInternalServerError)
		return
	}
	templReqs := make([]templates.RequestInfo, len(reqs))
	for i, req := range reqs {
		var hdrs map[string][]string
		_ = json.Unmarshal(req.Headers, &hdrs)
		templReqs[i] = templates.RequestInfo{
			Timestamp: req.Timestamp.Format(time.RFC3339),
			Headers:   hdrs,
			Body:      req.Body,
		}
	}
	templates.Listener(uuid, templReqs).Render(r.Context(), w)
}

func (a *App) wsHandler(w http.ResponseWriter, r *http.Request) {
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
	_, bufrw, err := hijacker.Hijack()
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
}

type websocketConn struct {
	send  chan string
	close chan struct{}
	w     http.ResponseWriter
	r     *http.Request
}

var wsConns = struct {
	sync.RWMutex
	data map[string]map[string]map[*websocketConn]struct{}
}{data: make(map[string]map[string]map[*websocketConn]struct{})}

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

package app

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	"encoding/base64"
	"crypto/sha1"

dbsqlc "webhooktester/db/sqlc"
	"webhooktester/templates"
)

func computeAcceptKey(key string) string {
	const magic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + magic))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
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
		newReq, err := a.Queries.CreateRequest(r.Context(), dbsqlc.CreateRequestParams{
			ListenerID: listener.ID,
			Headers:    headers,
			Body:       string(body),
		})
		if err != nil {
			http.Error(w, "Failed to save request", http.StatusInternalServerError)
			return
		}
		notifyWebSocketClients(uuid, newReq)
		w.WriteHeader(http.StatusOK)
		return
	}
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
	// Enforce JWT authentication for WebSocket
	username := getUsername(r)
	if username == "" {
		http.Error(w, "Unauthorized: missing or expired JWT", http.StatusUnauthorized)
		return
	}
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
	key := r.Header.Get("Sec-WebSocket-Key")
	accept := computeAcceptKey(key)
	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: websocket\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n")
	bufrw.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
	bufrw.Flush()

		// Register this connection for push updates
	registerWebSocketClient(uuid, bufrw, conn)
}

// --- WebSocket push infrastructure ---

// global map of uuid to list of clients
var wsClients = make(map[string][]*wsClient)
type wsClient struct {
	w    *bufio.ReadWriter
	conn net.Conn
}

func registerWebSocketClient(uuid string, w *bufio.ReadWriter, conn net.Conn) {
	client := &wsClient{w: w, conn: conn}
	addClient(uuid, client)
	defer removeClient(uuid, client)
	log.Printf("[WS] Registered client for uuid=%s", uuid)
	// Block until connection closes
	buf := make([]byte, 1)
	conn.Read(buf) // will unblock on close
	log.Printf("[WS] Connection closed for uuid=%s", uuid)
}

func addClient(uuid string, client *wsClient) {
	// Not thread safe, but works for single-threaded Go HTTP
	wsClients[uuid] = append(wsClients[uuid], client)
}

func removeClient(uuid string, client *wsClient) {
	clients := wsClients[uuid]
	for i, c := range clients {
		if c == client {
			wsClients[uuid] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
}

// Call this after saving a new request
func notifyWebSocketClients(uuid string, req dbsqlc.Request) {
	msg, _ := json.Marshal(map[string]interface{}{
		"Timestamp": req.Timestamp.Format(time.RFC3339),
		"Headers":   json.RawMessage(req.Headers),
		"Body":      req.Body,
	})
	log.Printf("[WS] Notifying %d clients for uuid=%s", len(wsClients[uuid]), uuid)
	for _, client := range wsClients[uuid] {
		err := writeWebSocketFrame(client.w, msg)
		if err != nil {
			log.Printf("[WS] Error writing to client: %v", err)
		}
	}
}

// writeWebSocketFrame writes a text frame to the WebSocket connection (RFC6455, minimal)
func writeWebSocketFrame(w *bufio.ReadWriter, payload []byte) error {
	defer func() {
		_ = recover() // avoid panic if connection is closed
	}()
	w.WriteByte(0x81) // FIN + text frame
	if len(payload) < 126 {
		w.WriteByte(byte(len(payload)))
	} else if len(payload) < 65536 {
		w.WriteByte(126)
		w.WriteByte(byte(len(payload) >> 8))
		w.WriteByte(byte(len(payload)))
	} else {
		w.WriteByte(127)
		for i := 7; i >= 0; i-- {
			w.WriteByte(byte(len(payload) >> uint(8*i)))
		}
	}
	_, err := w.Write(payload)
	if err != nil {
		return err
	}
	return w.Flush()
}

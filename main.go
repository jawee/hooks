package main

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"

dbsqlc "webhooktester/db/sqlc"
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
	// For websocket conns: user -> uuid -> set of conns
	wsConns = struct {
		sync.RWMutex
		data map[string]map[string]map[*websocketConn]struct{}
	}{data: make(map[string]map[string]map[*websocketConn]struct{})}
)

// Deprecated: queries is only used by getUsernameFromSession for withAuth. Use App.Queries elsewhere.
var queries *dbsqlc.Queries


type Config struct {
	DBHost, DBPort, DBUser, DBPassword, DBName, DBSSLMode string
	DemoUsername, DemoPassword string
}

type App struct {
	Config  Config
	DB      *sql.DB
	Queries *dbsqlc.Queries
}

func NewConfigFromEnv() Config {
	_ = godotenv.Load()
	return Config{
		DBHost:      os.Getenv("DB_HOST"),
		DBPort:      os.Getenv("DB_PORT"),
		DBUser:      os.Getenv("DB_USER"),
		DBPassword:  os.Getenv("DB_PASSWORD"),
		DBName:      os.Getenv("DB_NAME"),
		DBSSLMode:   os.Getenv("DB_SSLMODE"),
		DemoUsername: os.Getenv("DEMO_USERNAME"),
		DemoPassword: os.Getenv("DEMO_PASSWORD"),
	}
}

func NewApp(cfg Config) (*App, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("Database not reachable: %w", err)
	}
	return &App{
		Config:  cfg,
		DB:      db,
		Queries: dbsqlc.New(db),
	}, nil
}

func (a *App) setupDemoUser() error {
	if a.Config.DemoUsername != "" && a.Config.DemoPassword != "" {
		_, err := a.Queries.GetUserByUsername(context.Background(), a.Config.DemoUsername)
		if err != nil {
			if err == sql.ErrNoRows {
				_, err := a.Queries.CreateUser(context.Background(), dbsqlc.CreateUserParams{
					Username:     a.Config.DemoUsername,
					PasswordHash: a.Config.DemoPassword, // In production, hash this!
				})
				if err != nil {
					return fmt.Errorf("Failed to create demo user: %w", err)
				}
				log.Printf("[INFO] Demo user '%s' created", a.Config.DemoUsername)
			} else {
				return fmt.Errorf("Failed to check demo user: %w", err)
			}
		} else {
			log.Printf("[INFO] Demo user '%s' already exists", a.Config.DemoUsername)
		}
	}
	return nil
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/register", a.registerHandler)
	mux.HandleFunc("/login", a.loginHandler)
	mux.HandleFunc("/logout", a.logoutHandler)
	mux.HandleFunc("/", withAuth(a.indexHandler))
	mux.HandleFunc("/create-listener", withAuth(a.createListenerHandler))
	mux.HandleFunc("/listener/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			withAuth(a.listenerHandler)(w, r)
		} else {
			a.listenerHandler(w, r)
		}
	})
	mux.HandleFunc("/ws/", withAuth(a.wsHandler))
	return mux
}

func (a *App) Run(addr string) error {
	log.Println("Server started at http://" + addr)
	return http.ListenAndServe(addr, a.routes())
}

func main() {
	cfg := NewConfigFromEnv()
	app, err := NewApp(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := app.setupDemoUser(); err != nil {
		log.Fatal(err)
	}
	log.Fatal(app.Run(":8080"))
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
	// This function is now only used by withAuth, which is not a method on App.
	// It still uses the global queries variable for now.

	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}
	sess, err := queries.GetSessionByID(r.Context(), cookie.Value)
	if err != nil {
		return ""
	}
	user, err := queries.GetUserByID(r.Context(), sess.UserID)
	if err != nil {
		return ""
	}
	return user.Username
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
	// Check if user exists
	_, err := a.Queries.GetUserByUsername(r.Context(), username)
	if err == nil {
		templates.Register("Username already exists").Render(r.Context(), w)
		return
	}
	if err != sql.ErrNoRows {
		templates.Register("Internal error").Render(r.Context(), w)
		return
	}
	// Create user (store password as-is; hash in production!)
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
	// Set session in DB
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
	user, err := queries.GetUserByUsername(r.Context(), username)
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
	user, err := queries.GetUserByUsername(r.Context(), username)
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
		// Broadcast to websockets for real-time update
		wsConns.RLock()
		for conn := range wsConns.data {
			if userMap, ok := wsConns.data[conn]; ok {
				if uuidMap, ok := userMap[uuid]; ok {
					for ws := range uuidMap {
						var hdrs map[string][]string
						_ = json.Unmarshal(headers, &hdrs)
						msg := templates.RequestInfo{
							Timestamp: time.Now().Format(time.RFC3339),
							Headers:   hdrs,
							Body:      string(body),
						}
						select {
						case ws.send <- string(mustJSON(msg)):
						default:
						}
					}
				}
			}
		}
		wsConns.RUnlock()
		w.WriteHeader(http.StatusOK)
		return
	}
	// Auth required for GET
	username := getUsername(r)
	if username == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := queries.GetUserByUsername(r.Context(), username)
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
func (a *App) wsHandler(w http.ResponseWriter, r *http.Request) {
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

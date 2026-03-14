package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"

dbsqlc "webhooktester/db/sqlc"
)

type Config struct {
	DBHost, DBPort, DBUser, DBPassword, DBName, DBSSLMode string
	DemoUsername, DemoPassword string
}

type QueriesInterface interface {
	GetUserByUsername(ctx context.Context, username string) (dbsqlc.User, error)
	CreateUser(ctx context.Context, params dbsqlc.CreateUserParams) (dbsqlc.User, error)
	GetListenersByUser(ctx context.Context, userID int32) ([]dbsqlc.Listener, error)
	CreateSession(ctx context.Context, arg dbsqlc.CreateSessionParams) (dbsqlc.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	GetSessionByID(ctx context.Context, sessionID string) (dbsqlc.Session, error)
	CreateListener(ctx context.Context, arg dbsqlc.CreateListenerParams) (dbsqlc.Listener, error)
	GetListenerByUUID(ctx context.Context, uuid string) (dbsqlc.Listener, error)
	CreateRequest(ctx context.Context, arg dbsqlc.CreateRequestParams) (dbsqlc.Request, error)
	GetRequestsByListener(ctx context.Context, listenerID int32) ([]dbsqlc.Request, error)
}

type App struct {
	Config  Config
	DB      *sql.DB
	Queries QueriesInterface
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

// Run starts the HTTP server on the given address.
func (a *App) Run(addr string) error {
	log.Println("Server started at http://" + addr)
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/register", a.registerHandler)
	mux.HandleFunc("/login", a.loginHandler)
	mux.HandleFunc("/logout", a.logoutHandler)
	mux.HandleFunc("/", a.withSession(a.indexHandler))
	mux.HandleFunc("/create-listener", a.withSession(a.createListenerHandler))
	mux.HandleFunc("/listener/", a.withSession(a.listenerHandler))
	mux.HandleFunc("/ws/", a.withSession(a.wsHandler))
	return http.ListenAndServe(addr, mux)
}

func (a *App) SetupDemoUser() error {
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

// withSession is middleware that loads the session and injects username into context.
func (a *App) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err == nil {
			sess, err := a.Queries.GetSessionByID(r.Context(), cookie.Value)
			if err == nil {
				// Find user by ID
				userRows, err := a.DB.QueryContext(r.Context(), "SELECT username FROM users WHERE id = $1", sess.UserID)
				if err == nil && userRows.Next() {
					var username string
					userRows.Scan(&username)
					r = r.WithContext(contextWithUsername(r.Context(), username))
				}
				userRows.Close()
			}
		}
		next(w, r)
	}
}


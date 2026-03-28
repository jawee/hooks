package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"

dbsqlc "webhooktester/db/sqlc"
)

type Config struct {
	DBHost, DBPort, DBUser, DBPassword, DBName, DBSSLMode string
	DemoUsername, DemoPassword string
	JWTSecret string
	JWTLifetimeMinutes int
	RefreshTokenLifetimeHours int
}

type QueriesInterface interface {
	GetUserByUsername(ctx context.Context, username string) (dbsqlc.User, error)
	GetUserByID(ctx context.Context, id int32) (dbsqlc.User, error)
	CreateUser(ctx context.Context, params dbsqlc.CreateUserParams) (dbsqlc.User, error)
	GetListenersByUser(ctx context.Context, userID int32) ([]dbsqlc.Listener, error)
	CreateSession(ctx context.Context, arg dbsqlc.CreateSessionParams) (dbsqlc.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	GetSessionByID(ctx context.Context, sessionID string) (dbsqlc.Session, error)
	CreateListener(ctx context.Context, arg dbsqlc.CreateListenerParams) (dbsqlc.Listener, error)
	GetListenerByUUID(ctx context.Context, uuid string) (dbsqlc.Listener, error)
	CreateRequest(ctx context.Context, arg dbsqlc.CreateRequestParams) (dbsqlc.Request, error)
	GetRequestsByListener(ctx context.Context, listenerID int32) ([]dbsqlc.Request, error)
	UpdateListenerName(ctx context.Context, arg dbsqlc.UpdateListenerNameParams) error
	// Refresh token methods
	CreateRefreshToken(ctx context.Context, arg dbsqlc.CreateRefreshTokenParams) (dbsqlc.RefreshToken, error)
	GetRefreshToken(ctx context.Context, token string) (dbsqlc.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteUserRefreshTokens(ctx context.Context, userID int32) error
}

type App struct {
	Config  Config
	DB      *sql.DB
	Queries QueriesInterface
}

func NewConfigFromEnv() Config {
	_ = godotenv.Load()
	jwtLifetime := 5
	refreshLifetime := 24
	if v := os.Getenv("JWT_LIFETIME_MINUTES"); v != "" {
		fmt.Sscanf(v, "%d", &jwtLifetime)
	}
	if v := os.Getenv("REFRESH_TOKEN_LIFETIME_HOURS"); v != "" {
		fmt.Sscanf(v, "%d", &refreshLifetime)
	}
	return Config{
		DBHost:      os.Getenv("DB_HOST"),
		DBPort:      os.Getenv("DB_PORT"),
		DBUser:      os.Getenv("DB_USER"),
		DBPassword:  os.Getenv("DB_PASSWORD"),
		DBName:      os.Getenv("DB_NAME"),
		DBSSLMode:   os.Getenv("DB_SSLMODE"),
		DemoUsername: os.Getenv("DEMO_USERNAME"),
		DemoPassword: os.Getenv("DEMO_PASSWORD"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTLifetimeMinutes: jwtLifetime,
		RefreshTokenLifetimeHours: refreshLifetime,
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
	slog.Info("Server started", "url", "http://"+addr)
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/register", a.registerHandler)
	mux.HandleFunc("/login", a.loginHandler)
	mux.HandleFunc("/logout", a.logoutHandler)
	mux.HandleFunc("/refresh", a.refreshHandler)
	mux.HandleFunc("/", a.withJWT(a.indexHandler))
	mux.HandleFunc("/create-listener", a.withJWT(a.createListenerHandler))
	mux.HandleFunc("/listener/", a.withJWT(a.listenerHandler))
	mux.HandleFunc("/ws/", a.withJWT(a.wsHandler))
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
				slog.Info("Demo user created", "username", a.Config.DemoUsername)
			} else {
				return fmt.Errorf("Failed to check demo user: %w", err)
			}
		} else {
			slog.Info("Demo user exists", "username", a.Config.DemoUsername)
		}
	}
	return nil
}

// withJWT is middleware that validates JWT and injects username into context.
func (a *App) withJWT(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("jwt")
		slog.Debug("withJWT: checking JWT cookie", "cookie", cookie, "err", err)
		if err == nil {
			claims, err := ParseJWT(a.Config.JWTSecret, cookie.Value)
			slog.Debug("withJWT: parsed JWT", "claims", claims, "err", err)
			if err == nil {
				userID, ok := claims["user_id"].(float64)
				slog.Debug("withJWT: userID from claims", "userID", userID, "ok", ok)
				if ok {
					user, err := a.Queries.GetUserByID(r.Context(), int32(userID))
					slog.Debug("withJWT: fetched user by ID", "user", user, "err", err)
					if err == nil {
						r = r.WithContext(contextWithUsername(r.Context(), user.Username))
						// Also set username cookie for legacy handlers
						http.SetCookie(w, &http.Cookie{Name: "username", Value: user.Username, Path: "/", HttpOnly: true})
					}
				}
			}
		}
		next(w, r)
	}
}


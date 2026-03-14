package app

import (
	"context"
	"database/sql"
	"net/http"

dbsqlc "webhooktester/db/sqlc"
	"webhooktester/templates"
	"encoding/hex"
	"crypto/rand"
)

const usernameKey = contextKey("username")

type contextKey string

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

func newUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
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

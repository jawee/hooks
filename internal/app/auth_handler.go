package app

import (
	"context"
	"database/sql"
	"net/http"
	"time"

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
	// Generate JWT
	jwtToken, err := GenerateJWT(a.Config.JWTSecret, user.ID, a.Config.JWTLifetimeMinutes)
	if err != nil {
		templates.Login("Internal error").Render(r.Context(), w)
		return
	}
	// Generate refresh token
	refreshToken := newUUID()
	expiresAt := time.Now().Add(time.Duration(a.Config.RefreshTokenLifetimeHours) * time.Hour).Unix()
	_, err = a.Queries.CreateRefreshToken(r.Context(), dbsqlc.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		templates.Login("Internal error").Render(r.Context(), w)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "jwt", Value: jwtToken, Path: "/", HttpOnly: true, MaxAge: a.Config.JWTLifetimeMinutes * 60})
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: refreshToken, Path: "/", HttpOnly: true, MaxAge: a.Config.RefreshTokenLifetimeHours * 3600})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) refreshHandler(w http.ResponseWriter, r *http.Request) {
	refreshCookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "No refresh token", http.StatusUnauthorized)
		return
	}
	token, err := a.Queries.GetRefreshToken(r.Context(), refreshCookie.Value)
	if err != nil || token.ExpiresAt < time.Now().Unix() {
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}
	// Issue new JWT and refresh token
	jwtToken, err := GenerateJWT(a.Config.JWTSecret, token.UserID, a.Config.JWTLifetimeMinutes)
	if err != nil {
		http.Error(w, "Failed to generate JWT", http.StatusInternalServerError)
		return
	}
	newRefresh := newUUID()
	expiresAt := time.Now().Add(time.Duration(a.Config.RefreshTokenLifetimeHours) * time.Hour).Unix()
	_, err = a.Queries.CreateRefreshToken(r.Context(), dbsqlc.CreateRefreshTokenParams{
		Token:     newRefresh,
		UserID:    token.UserID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		http.Error(w, "Failed to create refresh token", http.StatusInternalServerError)
		return
	}
	_ = a.Queries.DeleteRefreshToken(r.Context(), refreshCookie.Value)
	http.SetCookie(w, &http.Cookie{Name: "jwt", Value: jwtToken, Path: "/", HttpOnly: true, MaxAge: a.Config.JWTLifetimeMinutes * 60})
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: newRefresh, Path: "/", HttpOnly: true, MaxAge: a.Config.RefreshTokenLifetimeHours * 3600})
	w.WriteHeader(http.StatusOK)
}

func (a *App) logoutHandler(w http.ResponseWriter, r *http.Request) {
	refreshCookie, err := r.Cookie("refresh_token")
	if err == nil {
		_ = a.Queries.DeleteRefreshToken(r.Context(), refreshCookie.Value)
		http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	}
	http.SetCookie(w, &http.Cookie{Name: "jwt", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

package app

import (
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"testing"
	dbsqlc "webhooktester/db/sqlc"
)

// TestLoginRedirectLoop reproduces the login redirect loop bug.
func TestLoginRedirectLoop(t *testing.T) {
	mockQueries := &MockQueries{}
	user := dbsqlc.User{ID: 1, Username: "bob", PasswordHash: "secret"}
	mockQueries.On("GetUserByUsername", mock.Anything, "bob").Return(user, nil)
	mockQueries.On("GetUserByID", mock.Anything, int32(1)).Return(user, nil)
	mockQueries.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(dbsqlc.RefreshToken{}, nil)
	mockQueries.On("GetListenersByUser", mock.Anything, int32(1)).Return([]dbsqlc.Listener{}, nil)
	app := &App{Queries: mockQueries, Config: Config{JWTSecret: "testsecret", JWTLifetimeMinutes: 5, RefreshTokenLifetimeHours: 24}}

	// Simulate login to get JWT cookie
	loginReq := httptest.NewRequest(http.MethodPost, "/login", nil)
	loginReq.PostForm = map[string][]string{"username": {"bob"}, "password": {"secret"}}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginW := httptest.NewRecorder()
	app.loginHandler(loginW, loginReq)
	if loginW.Code != http.StatusSeeOther {
		t.Fatalf("expected login redirect, got %d", loginW.Code)
	}
	jwtCookie := loginW.Result().Cookies()[0]

	// Now try to access index with JWT cookie
	indexReq := httptest.NewRequest(http.MethodGet, "/", nil)
	indexReq.AddCookie(jwtCookie)
	indexW := httptest.NewRecorder()
	app.withJWT(app.indexHandler)(indexW, indexReq)
	if indexW.Code == http.StatusSeeOther && indexW.Header().Get("Location") == "/login" {
		t.Errorf("redirect loop: still redirected to /login after login")
	}
}

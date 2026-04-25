package app

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	dbsqlc "webhooktester/db/sqlc"
)

func TestRegisterHandler_ExistingUsername(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{ID: 1, Username: "bob"}, nil)
	app := &App{Queries: mock}
	form := strings.NewReader("username=bob&password=pass")
	req := httptest.NewRequest(http.MethodPost, "/register", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.registerHandler(rw, req)
	assert.Contains(t, rw.Body.String(), "Username already exists")
}

func TestRegisterHandler_Success(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{}, sql.ErrNoRows)
	mock.On("CreateUser", mock2.Anything, mock2.Anything).Return(dbsqlc.User{}, nil)
	app := &App{Queries: mock}
	form := strings.NewReader("username=bob&password=pass")
	req := httptest.NewRequest(http.MethodPost, "/register", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.registerHandler(rw, req)
	assert.Equal(t, http.StatusSeeOther, rw.Code)
}

func TestLoginHandler_WrongPassword(t *testing.T) {
	mock := &MockQueries{}
	user := dbsqlc.User{ID: 1, Username: "bob", PasswordHash: "pass"}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(user, nil)
	app := &App{Queries: mock, Config: Config{JWTSecret: "test", JWTLifetimeMinutes: 5, RefreshTokenLifetimeHours: 1}}
	form := strings.NewReader("username=bob&password=wrong")
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.loginHandler(rw, req)
	assert.Contains(t, rw.Body.String(), "Invalid credentials")
}

func TestLoginHandler_NonExistingUser(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{}, assert.AnError)
	app := &App{Queries: mock, Config: Config{JWTSecret: "test", JWTLifetimeMinutes: 5, RefreshTokenLifetimeHours: 1}}
	form := strings.NewReader("username=bob&password=pass")
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.loginHandler(rw, req)
	assert.Contains(t, rw.Body.String(), "Invalid credentials")
}

func TestLoginHandler_Success(t *testing.T) {
	mock := &MockQueries{}
	user := dbsqlc.User{ID: 1, Username: "bob", PasswordHash: "pass"}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(user, nil)
	mock.On("CreateRefreshToken", mock2.Anything, mock2.Anything).Return(dbsqlc.RefreshToken{}, nil)
	app := &App{Queries: mock, Config: Config{JWTSecret: "test", JWTLifetimeMinutes: 5, RefreshTokenLifetimeHours: 1}}
	form := strings.NewReader("username=bob&password=pass")
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.loginHandler(rw, req)
	assert.Equal(t, http.StatusSeeOther, rw.Code)
}

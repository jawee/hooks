package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"database/sql"
	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	dbsqlc "webhooktester/db/sqlc"
)

func TestRegisterHandler_EmptyForm(t *testing.T) {
	mock := &MockQueries{}
	app := &App{Queries: mock}
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.registerHandler(rw, req)
	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rw.Code)
	}
	if !strings.Contains(rw.Body.String(), "Username and password required") {
		t.Errorf("expected error message, got %q", rw.Body.String())
	}
}

func TestRegisterHandler_UserExists(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{ID: 1, Username: "bob"}, nil)
	app := &App{Queries: mock}
	form := "username=bob&password=secret"
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.registerHandler(rw, req)
	if !strings.Contains(rw.Body.String(), "Username already exists") {
		t.Errorf("expected user exists error, got %q", rw.Body.String())
	}
}

func TestRegisterHandler_DBError(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{}, sql.ErrNoRows)
	mock.On("CreateUser", mock2.Anything, mock2.Anything).Return(dbsqlc.User{}, assert.AnError)
	app := &App{Queries: mock}
	form := "username=bob&password=secret"
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.registerHandler(rw, req)
	if !strings.Contains(rw.Body.String(), "Failed to create user") {
		t.Errorf("expected create user error, got %q", rw.Body.String())
	}
}

func TestRegisterHandler_Success(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{}, sql.ErrNoRows)
	mock.On("CreateUser", mock2.Anything, mock2.Anything).Return(dbsqlc.User{ID: 1, Username: "bob"}, nil)
mock.On("CreateRefreshToken", mock2.Anything, mock2.Anything).Return(dbsqlc.RefreshToken{}, nil)
	app := &App{Queries: mock}
	form := "username=bob&password=secret"
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.registerHandler(rw, req)
	if rw.Code != http.StatusSeeOther {
		t.Errorf("expected redirect to login, got %d", rw.Code)
	}
}

func TestLoginHandler_EmptyForm(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "").Return(dbsqlc.User{}, sql.ErrNoRows)
	app := &App{Queries: mock}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.loginHandler(rw, req)
	if !strings.Contains(rw.Body.String(), "Invalid credentials") {
		t.Errorf("expected error message, got %q", rw.Body.String())
	}
}

func TestLoginHandler_UserNotFound(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{}, sql.ErrNoRows)
	app := &App{Queries: mock}
	form := "username=bob&password=secret"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.loginHandler(rw, req)
	if !strings.Contains(rw.Body.String(), "Invalid credentials") {
		t.Errorf("expected invalid login, got %q", rw.Body.String())
	}
}

func TestLoginHandler_DBError(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{}, assert.AnError)
	app := &App{Queries: mock}
	form := "username=bob&password=secret"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.loginHandler(rw, req)
	if !strings.Contains(rw.Body.String(), "Invalid credentials") {
		t.Errorf("expected internal error, got %q", rw.Body.String())
	}
}

func TestLoginHandler_Success(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{ID: 1, Username: "bob", PasswordHash: "secret"}, nil)
	mock.On("CreateRefreshToken", mock2.Anything, mock2.Anything).Return(dbsqlc.RefreshToken{}, nil)
	app := &App{Queries: mock, Config: Config{JWTSecret: "testsecret", JWTLifetimeMinutes: 5, RefreshTokenLifetimeHours: 24}}
	form := "username=bob&password=secret"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	app.loginHandler(rw, req)
	if rw.Code != http.StatusSeeOther {
		t.Errorf("expected redirect, got %d", rw.Code)
	}
}



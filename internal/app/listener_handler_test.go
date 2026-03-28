package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"fmt"
	"strings"
	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	dbsqlc "webhooktester/db/sqlc"
)

func TestIndexHandler_Unauthenticated(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, mock2.Anything).Return(dbsqlc.User{}, fmt.Errorf("user not found"))
	app := &App{Queries: mock}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rw := httptest.NewRecorder()
	app.indexHandler(rw, req)
	if rw.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rw.Code)
	}
	if loc := rw.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestIndexHandler_Success(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, mock2.Anything).Return(dbsqlc.User{ID: 1, Username: "bob"}, nil)
	mock.On("GetListenersByUser", mock2.Anything, int32(1)).Return([]dbsqlc.Listener{{Uuid: "abc"}}, nil)
	app := &App{Queries: mock}
	// Simulate authenticated user by setting username cookie
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "username", Value: "bob"})
	rw := httptest.NewRecorder()
	app.indexHandler(rw, req)
	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rw.Code)
	}
	if !strings.Contains(rw.Body.String(), "abc") {
		t.Errorf("expected listener uuid, got %q", rw.Body.String())
	}
}

func TestIndexHandler_DBError(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, mock2.Anything).Return(dbsqlc.User{ID: 1, Username: "bob"}, nil)
	mock.On("GetListenersByUser", mock2.Anything, int32(1)).Return([]dbsqlc.Listener{}, assert.AnError)
	app := &App{Queries: mock}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rw := httptest.NewRecorder()
	app.indexHandler(rw, req)
	if rw.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rw.Code)
	}
	if loc := rw.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestCreateListenerHandler_Unauthenticated(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, mock2.Anything).Return(dbsqlc.User{}, fmt.Errorf("user not found"))
	app := &App{Queries: mock}
	req := httptest.NewRequest(http.MethodPost, "/listener", nil)
	rw := httptest.NewRecorder()
	app.createListenerHandler(rw, req)
	if rw.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rw.Code)
	}
	if loc := rw.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestCreateListenerHandler_Success(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, mock2.Anything).Return(dbsqlc.User{ID: 1, Username: "bob"}, nil)
	mock.On("CreateListener", mock2.Anything, mock2.Anything).Return(dbsqlc.Listener{Uuid: "abc", UserID: 1}, nil)
	app := &App{Queries: mock}
	req := httptest.NewRequest(http.MethodPost, "/listener", nil)
	rw := httptest.NewRecorder()
	app.createListenerHandler(rw, req)
	if rw.Code != http.StatusSeeOther {
		t.Errorf("expected redirect, got %d", rw.Code)
	}
}

func TestCreateListenerHandler_DBError(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, mock2.Anything).Return(dbsqlc.User{ID: 1, Username: "bob"}, nil)
	mock.On("CreateListener", mock2.Anything, mock2.Anything).Return(dbsqlc.Listener{}, assert.AnError)
	app := &App{Queries: mock}
	req := httptest.NewRequest(http.MethodPost, "/listener", nil)
	rw := httptest.NewRecorder()
	app.createListenerHandler(rw, req)
	if rw.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rw.Code)
	}
}



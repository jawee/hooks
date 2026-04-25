package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	dbsqlc "webhooktester/db/sqlc"
)

func TestListenersHandler_Unauthenticated(t *testing.T) {
	app := &App{Queries: &MockQueries{}}
	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/listeners", nil)
	app.listenersHandler(rw, req)
	assert.Equal(t, http.StatusSeeOther, rw.Code)
}

func TestListenerRESTHandler_Forbidden(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{ID: 1, Username: "bob"}, nil)
	// Listener belongs to another user (ID: 2)
	mock.On("GetListenerByUUID", mock2.Anything, "uuid-123").Return(dbsqlc.Listener{Uuid: "uuid-123", UserID: 2}, nil)
	app := &App{Queries: mock}
	req := httptest.NewRequest(http.MethodGet, "/listeners/uuid-123", nil)
	req.AddCookie(&http.Cookie{Name: "username", Value: "bob"})
	rw := httptest.NewRecorder()
	app.listenerRESTHandler(rw, req)
	assert.Equal(t, http.StatusNotFound, rw.Code)
}

func TestListenersHandler_Authenticated(t *testing.T) {
	mock := &MockQueries{}
	mock.On("GetUserByUsername", mock2.Anything, "bob").Return(dbsqlc.User{ID: 1, Username: "bob"}, nil)
	mock.On("GetListenersByUser", mock2.Anything, int32(1)).Return([]dbsqlc.Listener{}, nil)
	app := &App{Queries: mock}
	req := httptest.NewRequest(http.MethodGet, "/listeners", nil)
	req.AddCookie(&http.Cookie{Name: "username", Value: "bob"})
	rw := httptest.NewRecorder()
	app.listenersHandler(rw, req)
	assert.Equal(t, http.StatusOK, rw.Code)
}

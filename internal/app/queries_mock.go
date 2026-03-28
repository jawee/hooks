package app

import (
	"context"
	"github.com/stretchr/testify/mock"

	dbsqlc "webhooktester/db/sqlc"
)

type MockQueries struct {
	UpdateListenerNameFunc func(ctx context.Context, arg dbsqlc.UpdateListenerNameParams) error

	mock.Mock
}

func (m *MockQueries) GetUserByID(ctx context.Context, id int32) (dbsqlc.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(dbsqlc.User), args.Error(1)
}

func (m *MockQueries) CreateRefreshToken(ctx context.Context, arg dbsqlc.CreateRefreshTokenParams) (dbsqlc.RefreshToken, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbsqlc.RefreshToken), args.Error(1)
}

func (m *MockQueries) GetRefreshToken(ctx context.Context, token string) (dbsqlc.RefreshToken, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(dbsqlc.RefreshToken), args.Error(1)
}

func (m *MockQueries) DeleteRefreshToken(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockQueries) DeleteUserRefreshTokens(ctx context.Context, userID int32) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockQueries) GetUserByUsername(ctx context.Context, username string) (dbsqlc.User, error) {
	args := m.Called(ctx, username)
	return args.Get(0).(dbsqlc.User), args.Error(1)
}

func (m *MockQueries) CreateUser(ctx context.Context, params dbsqlc.CreateUserParams) (dbsqlc.User, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(dbsqlc.User), args.Error(1)
}

func (m *MockQueries) GetListenersByUser(ctx context.Context, userID int32) ([]dbsqlc.Listener, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]dbsqlc.Listener), args.Error(1)
}

func (m *MockQueries) CreateSession(ctx context.Context, arg dbsqlc.CreateSessionParams) (dbsqlc.Session, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbsqlc.Session), args.Error(1)
}

func (m *MockQueries) DeleteSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockQueries) GetSessionByID(ctx context.Context, sessionID string) (dbsqlc.Session, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(dbsqlc.Session), args.Error(1)
}

func (m *MockQueries) CreateListener(ctx context.Context, arg dbsqlc.CreateListenerParams) (dbsqlc.Listener, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbsqlc.Listener), args.Error(1)
}

func (m *MockQueries) GetListenerByUUID(ctx context.Context, uuid string) (dbsqlc.Listener, error) {
	args := m.Called(ctx, uuid)
	return args.Get(0).(dbsqlc.Listener), args.Error(1)
}

func (m *MockQueries) CreateRequest(ctx context.Context, arg dbsqlc.CreateRequestParams) (dbsqlc.Request, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbsqlc.Request), args.Error(1)
}

func (m *MockQueries) UpdateListenerName(ctx context.Context, arg dbsqlc.UpdateListenerNameParams) error {
	if m.UpdateListenerNameFunc != nil {
		return m.UpdateListenerNameFunc(ctx, arg)
	}
	return nil
}

func (m *MockQueries) GetRequestsByListener(ctx context.Context, listenerID int32) ([]dbsqlc.Request, error) {
	args := m.Called(ctx, listenerID)
	return args.Get(0).([]dbsqlc.Request), args.Error(1)
}

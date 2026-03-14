-- +goose Up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL
);

CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    session_id TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE listeners (
    id SERIAL PRIMARY KEY,
    uuid TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE requests (
    id SERIAL PRIMARY KEY,
    listener_id INTEGER NOT NULL REFERENCES listeners(id),
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    headers JSONB NOT NULL,
    body TEXT NOT NULL
);
-- +goose Down
DROP TABLE IF EXISTS requests;
DROP TABLE IF EXISTS listeners;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

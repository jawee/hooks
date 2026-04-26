# Copilot Instructions for This Repository

## Build, Test, and Lint Commands

- **Install dependencies:**
  ```bash
  go mod download
  make install-templ
  make install-air
  ```
- **Generate code from templates:**
  ```bash
  make generate
  ```
- **Build the application:**
  ```bash
  make build
  ```
- **Run the server (with hot reload):**
  ```bash
  make run
  ```
- **Run all tests:**
  ```bash
  make test
  ```
- **Run a single test file:**
  ```bash
  go test ./internal/app/auth_handler_test.go
  ```
- **Migrate database up:**
  ```bash
  make migrate-up
  ```
- **Migrate database down:**
  ```bash
  make migrate-down
  ```

## High-Level Architecture

- **Backend:** Go web server (main entry: `cmd/app/main.go`, core logic in `internal/app/`).
- **Frontend:** HTMX for interactivity, Tailwind CSS for styling, Templ for type-safe templates (`templates/`).
- **Persistence:** All business data in Postgres (see `.env.sample` for config).
- **WebSocket:** Real-time updates for webhook requests.
- **Per-user isolation:** Each user has unique webhook listeners and sessions.
- **Demo user:** Created on startup for quick testing (see `SetupDemoUser`).

## Key Conventions

- **Template Generation:** Always run `make generate` before building/running to ensure Go code is generated from `.templ` files.
- **Sessions:** Expire after 1 hour; only active WebSocket connections are in memory.
- **Testing:** Use Go's built-in test framework; test files are named `*_test.go`.
- **No automatic commits:** All commits must be explicitly requested by the user/agent.

---

If you need to add new conventions or update instructions, edit this file to help future Copilot sessions.

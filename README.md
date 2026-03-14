# Webhook Tester

A Go web server for testing webhooks with user authentication and per-user listener isolation.

## Features
- User registration and login (in-memory, no persistence)
- Per-user webhook listeners with unique UUIDs
- Real-time webhook request viewing with WebSocket updates
- Anonymous POST to webhook endpoints
- HTMX and Tailwind CSS for frontend
- Type-safe templates using [templ](https://templ.guide/)

## Getting Started

### Prerequisites
- Go 1.23+ (https://golang.org/dl/)
- templ CLI tool

### Installation

1. Clone the repository
2. Install dependencies:
```bash
go mod download
```

3. Install the templ CLI:
```bash
make install-templ
# or manually:
go install github.com/a-h/templ/cmd/templ@latest
```

### Running the Server

#### Development (with hot reload)
```bash
make install-air  # First time only
make run
```

#### Production
```bash
make build
./webhooktester
```

The server will start at http://localhost:8080

### Build Process

This project uses [templ](https://templ.guide/) for type-safe Go templates. Before building, you must generate the Go code from `.templ` files:

```bash
# Generate template code
make generate

# Build the application (automatically runs generate)
make build

# Run with hot reload (automatically runs generate)
make run
```

## Project Structure
- `main.go`: HTTP server, handlers, and in-memory data stores
- `templates/`: Templ template files (.templ) and generated Go code
- `templates/layout/`: Shared layout components
- `static/`: Frontend JavaScript assets
- `.air.toml`: Air configuration for hot reload
- `Makefile`: Build and development commands

## Usage

1. Register a new user or login with demo credentials (username: `demo`, password: `demo123`)
2. Create a new webhook listener
3. Send POST requests to the webhook URL
4. View incoming requests in real-time

## Notes
- All data is stored in-memory and lost on server restart
- Sessions expire after 1 hour
- WebSocket implementation is basic (for demo purposes)
- Templ generates `*_templ.go` files which are excluded from git but required for builds

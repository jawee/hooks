# Project Context for Copilot

## Purpose
A Go-based webhook testing tool with user authentication and per-user isolation. Each user can create, view, and manage their own webhook listeners. Webhook POSTs are anonymous, but only the owner can view the data.

## Main Technologies
- Go (net/http, templ)
- templ (type-safe Go templates)
- HTMX (frontend interactivity)
- Tailwind CSS (styling)

## Key Endpoints
- `/register`, `/login`, `/logout`: User authentication
- `/`: Dashboard (requires login)
- `/create-listener`: Create a new webhook listener (requires login)
- `/listener/{uuid}`: View listener (requires login for GET, anonymous POST)
- `/ws/{uuid}`: WebSocket for real-time updates (requires login)

## Security
- All user data is in-memory only (no persistence)
- Sessions are managed with secure cookies
- Each user can only see their own listeners

## Build Process
- Templates use templ, not html/template
- Run `templ generate` before building to generate Go code from .templ files
- Generated *_templ.go files are excluded from git
- Use `make generate` or `make build` (which auto-generates)

## Agent Guidance
- Always enforce user isolation and session checks
- Do not add persistent storage unless requested
- Use idiomatic Go and keep code readable
- Use Tailwind for all template styling
- When modifying templates, edit .templ files and regenerate

---

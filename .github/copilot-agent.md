# Copilot Agent Instructions

## Project Overview
This project is a Go web server for testing webhooks, with per-user authentication and listener isolation. Users can register, log in, and manage their own webhook receivers. Webhook POSTs are anonymous, but only the owner can view their listeners and received requests.

## Key Features
- User registration and login (in-memory, no persistence)
- Per-user isolation of webhook listeners
- Anonymous POST to /listener/{uuid} for webhook delivery
- HTMX and Tailwind for frontend interactivity and styling
- Debug logging for session and listener actions
- Type-safe templates using templ

## Coding Conventions
- Use idiomatic Go (gofmt, goimports)
- Use net/http for routing and middleware
- Use templ for all HTML rendering (not html/template)
- Always run `templ generate` before building or running
- Use sync.RWMutex for all shared in-memory data
- No persistence: all data is lost on server restart
- All authentication and session logic must be secure and simple

## Agent Instructions
- When adding new features, always consider user isolation and session security
- When editing templates, create or modify .templ files in templates/ directory
- After modifying .templ files, run `templ generate` to regenerate Go code
- When adding new endpoints, document them in README.md
- Do not add persistence unless explicitly requested
- Do not commit changes unless the user asks for it
- Generated *_templ.go files should not be committed (excluded via .gitignore)

## File Structure
- main.go: All backend logic, handlers, and in-memory stores
- templates/: Templ template files (.templ) and generated Go code (*_templ.go)
- templates/layout/: Shared layout components
- static/: Frontend JS and CSS assets
- .air.toml: Air config for hot reload
- README.md: Project documentation

---

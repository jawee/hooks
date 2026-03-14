# Copilot Skill: hooks Project

## Domain
Go web server, user authentication, webhook listener management, in-memory data, HTMX, Tailwind CSS, templ templates.

## Key Skills
- Implementing secure session-based authentication in Go
- Managing per-user in-memory data with sync.RWMutex
- Building type-safe templates with templ (not html/template)
- Generating Go code from .templ files using templ CLI
- Integrating HTMX for dynamic frontend updates
- Using Tailwind CSS for modern, accessible UI
- Debugging session and context issues in Go web apps

## Usage
- Use this skill file to inform Copilot of the project's domain and best practices
- When asked to add features, always consider user isolation and session security
- When editing templates, modify .templ files and run `templ generate`
- Always regenerate templates after .templ file changes
- Do not add persistence or external storage unless requested

---

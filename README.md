# Go HTMX Boilerplate

This is a minimal Go web server using HTMX for frontend interactivity.

## Features
- Serves static files and HTML templates
- Uses HTMX for dynamic content updates
- Example: Button fetches server time via HTMX

## Getting Started

1. Install Go (https://golang.org/dl/)
2. Run the server:

    go run main.go

3. Open http://localhost:8080 in your browser

## Project Structure
- main.go: Go HTTP server
- templates/: HTML templates
- static/: Static files (HTMX JS)

## Notes
- The included htmx.min.js is a placeholder. Replace with the latest from https://htmx.org for production use.

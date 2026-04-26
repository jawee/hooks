
.PHONY: generate build test run install-air install-templ css


generate:
	templ generate

css:
	npx tailwindcss-cli -o ./static/tailwind.css --minify

build: generate css
	go build -o $(APP) ./cmd/app/main.go

test:
	go test ./...

run: generate css
	air

install-air:
	go install github.com/cosmtrek/air@latest

install-templ:
	go install github.com/a-h/templ/cmd/templ@latest

migrate-up:
	set -a && source .env && set +a && DATABASE_URL="postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSLMODE" go run ./cmd/migrations/main.go

migrate-down:
	set -a && source .env && set +a && goose -dir db/migration postgres "host=$$DB_HOST port=$$DB_PORT user=$$DB_USER password=$$DB_PASSWORD dbname=$$DB_NAME sslmode=$$DB_SSLMODE" down

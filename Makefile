.PHONY: generate build test run install-air install-templ

generate:
	templ generate

build: generate
	go build -o $(APP) main.go

test:
	go test ./...

run: generate
	air

install-air:
	go install github.com/cosmtrek/air@latest

install-templ:
	go install github.com/a-h/templ/cmd/templ@latest

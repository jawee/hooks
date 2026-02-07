build:
	go build -o $(APP) main.go

test:
	go test ./...

run:
	air

install-air:
	go install github.com/cosmtrek/air@latest

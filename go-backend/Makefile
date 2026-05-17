.PHONY: run build tidy clean

## Download dependencies
tidy:
	go mod tidy

## Run in development mode
run: tidy
	go run ./cmd/main.go

## Build production binary
build: tidy
	go build -o ipms-server ./cmd/main.go
	@echo "✅ Binary: ./ipms-server"

## Run the built binary
start: build
	./ipms-server

## Clean build artifacts
clean:
	rm -f ipms-server ipms.db

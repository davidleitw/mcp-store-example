.PHONY: build build-server build-client run clean deps fmt test

# Build both server and client
build: build-server build-client

# Build the server
build-server:
	go build -o bin/product-server cmd/server/main.go

# Build the client
build-client:
	go build -o bin/product-client cmd/client/main.go

# Run the client (which will start the server)
run: build
	./bin/product-client

# Clean build artifacts
clean:
	rm -rf bin/

# Install dependencies
deps:
	go mod download

# Format code
fmt:
	go fmt ./...

# Run tests
test:
	go test ./... 
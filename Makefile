.PHONY: build run test clean install

# Build the server binary
build:
	go build -o bin/number-dispenser cmd/server/main.go

# Run the server
run: build
	./bin/number-dispenser -addr :6380 -data ./data

# Run tests
test:
	go test -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf data/
	rm -f coverage.out coverage.html

# Install dependencies
install:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o bin/number-dispenser-linux-amd64 cmd/server/main.go
	GOOS=linux GOARCH=arm64 go build -o bin/number-dispenser-linux-arm64 cmd/server/main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/number-dispenser-darwin-amd64 cmd/server/main.go
	GOOS=darwin GOARCH=arm64 go build -o bin/number-dispenser-darwin-arm64 cmd/server/main.go
	GOOS=windows GOARCH=amd64 go build -o bin/number-dispenser-windows-amd64.exe cmd/server/main.go


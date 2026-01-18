.PHONY: build clean test run-daemon run-tui install

# Build all binaries
build:
	go build -o bin/cosa ./cmd/cosa
	go build -o bin/cosad ./cmd/cosad

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f ~/.cosa/cosa.sock
	rm -f ~/.cosa/cosad.pid

# Run tests
test:
	go test ./...

# Start daemon in foreground
run-daemon: build
	./bin/cosa start -f

# Start TUI
run-tui: build
	./bin/cosa tui

# Install to $GOBIN
install:
	go install ./cmd/cosa
	go install ./cmd/cosad

# Stop daemon
stop:
	./bin/cosa stop || true

# Show status
status:
	./bin/cosa status

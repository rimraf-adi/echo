VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: all build test test-integration clean

all: build

build:
	@mkdir -p bin
	go build -buildvcs=false $(LDFLAGS) -o bin/echo ./cmd/echo

test:
	go test ./... -v -race -count=1

test-integration:
	go test ./tests/integration/... -v -race -count=1

clean:
	rm -rf bin/

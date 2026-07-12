MODULE     := github.com/rdu90/RPG
BIN_DIR    := bin
DIST_DIR   := dist
BINARY     := rpg
CMD_PKG    := ./cmd/rpg

PLATFORMS  := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all build run test test-verbose cover fmt fmt-write vet lint tidy check release clean help

all: build

build: ## Build the rpg binary into bin/
	go build -o $(BIN_DIR)/$(BINARY) $(CMD_PKG)

run: build ## Build and run the rpg binary
	./$(BIN_DIR)/$(BINARY)

test: ## Run the test suite
	go test ./...

test-verbose: ## Run the test suite with verbose output
	go test -v ./...

cover: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

fmt: ## Fail if any file is not gofmt-formatted
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needs to be run on:"; echo "$$unformatted"; \
		echo "run 'make fmt-write' to fix"; \
		exit 1; \
	fi

fmt-write: ## Reformat all files with gofmt
	gofmt -w .

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (pinned via the go.mod tool directive)
	go tool golangci-lint run ./...

tidy: ## Tidy go.mod/go.sum
	go mod tidy

check: fmt vet lint test ## Run everything CI should run before a merge

release: ## Cross-compile release binaries into dist/
	mkdir -p $(DIST_DIR)
	$(foreach p,$(PLATFORMS), \
		GOOS=$(word 1,$(subst /, ,$(p))) GOARCH=$(word 2,$(subst /, ,$(p))) \
		go build -o $(DIST_DIR)/$(BINARY)-$(word 1,$(subst /, ,$(p)))-$(word 2,$(subst /, ,$(p))) $(CMD_PKG);)

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out

help: ## List available targets
	@grep -hE '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*##"}; {printf "  %-14s %s\n", $$1, $$2}'

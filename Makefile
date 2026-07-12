MODULE     := github.com/rdu90/RPG
BIN_DIR    := bin
DIST_DIR   := dist
BINARY     := rpg
CMD_PKG    := ./cmd/rpg

PLATFORMS  := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all build run test test-verbose test-race cover cover-html fmt fmt-write vet lint tidy vuln check release clean help new-migration db-shell db-query saves

all: build

build: ## Build the rpg binary into bin/
	go build -o $(BIN_DIR)/$(BINARY) $(CMD_PKG)

run: build ## Build and run the rpg binary
	./$(BIN_DIR)/$(BINARY)

test: ## Run the test suite
	go test ./...

test-verbose: ## Run the test suite with verbose output
	go test -v ./...

test-race: ## Run the test suite with the race detector
	go test -race ./...

cover: ## Run tests with a function-level coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

cover-html: cover ## Run tests and open an HTML coverage report
	go tool cover -html=coverage.out

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

vuln: ## Check dependencies and the stdlib for known vulnerabilities
	go tool govulncheck ./...

check: fmt vet lint test ## Run everything CI should run before a merge

release: ## Cross-compile release binaries into dist/
	mkdir -p $(DIST_DIR)
	$(foreach p,$(PLATFORMS), \
		GOOS=$(word 1,$(subst /, ,$(p))) GOARCH=$(word 2,$(subst /, ,$(p))) \
		go build -o $(DIST_DIR)/$(BINARY)-$(word 1,$(subst /, ,$(p)))-$(word 2,$(subst /, ,$(p))) $(CMD_PKG);)

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out

new-migration: ## Scaffold the next goose migration (make new-migration NAME=add_x)
	@test -n "$(NAME)" || (echo "usage: make new-migration NAME=<description>" >&2; exit 1)
	./scripts/new_migration.sh "$(NAME)"

db-shell: ## Open a sqlite3 shell on a save, requires the sqlite3 CLI (make db-shell SAVE=name)
	@test -n "$(SAVE)" || (echo "usage: make db-shell SAVE=<save-name>" >&2; exit 1)
	./scripts/db_shell.sh "$(SAVE)"

db-query: ## Run one SQL statement against a save, no sqlite3 CLI required (make db-query SAVE=name SQL="select ...")
	@test -n "$(SAVE)" || (echo "usage: make db-query SAVE=<save-name> SQL=<statement>" >&2; exit 1)
	@test -n "$(SQL)" || (echo "usage: make db-query SAVE=<save-name> SQL=<statement>" >&2; exit 1)
	./scripts/db_query.sh "$(SAVE)" "$(SQL)"

saves: ## List local save files
	@ls -la "$${XDG_DATA_HOME:-$$HOME/.local/share}/rpg" 2>/dev/null || echo "no saves yet"

help: ## List available targets
	@grep -hE '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*##"}; {printf "  %-14s %s\n", $$1, $$2}'

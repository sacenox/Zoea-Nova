.PHONY: fmt build run test test-integration install clean db-reset-accounts check-upstream-version

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"
DB_PATH ?= $(HOME)/.zoea-nova/zoea.db
ACCOUNTS_EXPORT ?= accounts-backup.sql

fmt:
	go fmt ./...

build:
	go build $(LDFLAGS) -o bin/zoea ./cmd/zoea

run: build
	./bin/zoea

install: build
	mkdir -p $(HOME)/.zoea-nova/bin
	cp bin/zoea $(HOME)/.zoea-nova/bin/zoea

test:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-integration:
	go test -v ./internal/tui -run TestIntegration -count=1

check-upstream-version:
	@echo "Config upstream version:"
	@rg "upstream_version" config.toml
	@echo "Remote upstream version:"
	@curl -s https://www.spacemolt.com/api.md | rg -o "gameserver v[0-9.]+"

clean:
	rm -rf bin/ coverage.out

db-reset-accounts:
	mkdir -p $(HOME)/.zoea-nova
	test -f $(DB_PATH) && sqlite3 $(DB_PATH) "SELECT 'INSERT INTO accounts (username, password) VALUES (' || quote(username) || ',' || quote(password) || ');' FROM accounts;" > $(ACCOUNTS_EXPORT) || true
	rm -f $(DB_PATH) $(DB_PATH)-shm $(DB_PATH)-wal
	sqlite3 $(DB_PATH) < internal/store/schema.sql
	test -s $(ACCOUNTS_EXPORT) && sqlite3 $(DB_PATH) < $(ACCOUNTS_EXPORT) || true

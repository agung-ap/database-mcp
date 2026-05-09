.PHONY: build build-race build-linux run check test test-race test-integration test-coverage fmt lint lint-fix vet mocks generate docker-up docker-down docker-reset docker-logs clean vuln scan mod-tidy

# Single command: fmt + vet + lint + build
check: fmt vet lint build

build:
	go build -o bin/mcp-db-server ./cmd/server/...

build-race:
	go build -race -o bin/mcp-db-server-race ./cmd/server/...

build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/mcp-db-server-linux ./cmd/server/...

run:
	go run ./cmd/server/main.go

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

test:
	go test ./... -timeout 60s

test-race:
	go test -race ./... -timeout 60s

test-integration:
	go test ./... -tags=integration -timeout 120s

test-coverage:
	go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html

mocks:
	go generate ./...

generate:
	go generate ./...

docker-up:
	docker compose -f docker/docker-compose.test.yml up -d

docker-down:
	docker compose -f docker/docker-compose.test.yml down

docker-reset:
	docker compose -f docker/docker-compose.test.yml down -v && make docker-up

docker-logs:
	docker compose -f docker/docker-compose.test.yml logs -f

clean:
	rm -rf bin/ coverage.out coverage.html

# ── Security ──────────────────────────────────────────────────────────────────

## vuln: run govulncheck (call-graph aware); fails if any reachable CVE found
vuln:
	govulncheck ./...

## vuln-verbose: same as vuln but prints all imported-but-unreachable findings too
vuln-verbose:
	govulncheck -show verbose ./...

## scan: full security gate — vet + lint + vuln (run before every push)
scan: vet lint vuln

## mod-tidy: tidy module graph and verify go.sum is consistent
mod-tidy:
	go mod tidy
	go mod verify

.PHONY: dev down test lint tidy migrate-up migrate-down migrate-status

BE := crm_be

# --- dev environment (docker-compose.yml lands in issue #2) ---

dev:
	docker compose up --build

down:
	docker compose down

# --- backend ---

test:
	cd $(BE) && go test -race ./...

lint:
	cd $(BE) && golangci-lint run

tidy:
	cd $(BE) && go mod tidy

# --- migrations (cmd/migrate lands in issue #2) ---

migrate-up:
	cd $(BE) && go run ./cmd/migrate up

migrate-down:
	cd $(BE) && go run ./cmd/migrate down

migrate-status:
	cd $(BE) && go run ./cmd/migrate status

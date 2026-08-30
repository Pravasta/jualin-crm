.PHONY: dev down test lint tidy migrate-up migrate-down migrate-status \
	mobile-get mobile-analyze mobile-test mobile-run mobile-apk

BE := crm_be
MOBILE := crm_employee

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

# --- mobile (crm_employee lands in issue #69) ---
# All through `fvm` — the pinned 3.44.0 from crm_employee/.fvmrc, not
# whatever `flutter` happens to resolve to in PATH on this machine
# (Phase 5 TD §2, decision M2).

mobile-get:
	cd $(MOBILE) && fvm flutter pub get

mobile-analyze:
	cd $(MOBILE) && fvm flutter analyze

mobile-test:
	cd $(MOBILE) && fvm flutter test

mobile-run:
	cd $(MOBILE) && fvm flutter run

mobile-apk:
	cd $(MOBILE) && fvm flutter build apk --debug

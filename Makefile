# Common commands for the monorepo. Run `make help` for the list.

# postgresql@16 is "keg-only" in Homebrew: its binaries are not put on PATH,
# so they are called through their full path.
PG_FORMULA := postgresql@16
PG_BIN     := $(shell brew --prefix)/opt/$(PG_FORMULA)/bin

DEV_DB  := voice_translation
TEST_DB := voice_translation_test

.DEFAULT_GOAL := help

.PHONY: help
help: ## List the available commands
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-16s %s\n", $$1, $$2}'

# --- Postgres ---------------------------------------------------------------

.PHONY: db-install
db-install: ## One-time install of Postgres 16 through Homebrew
	brew install $(PG_FORMULA)

# `brew services run` starts Postgres without registering it to start at
# login, so it only uses RAM while you are working on this project.
.PHONY: db-up
db-up: ## Start Postgres and create the dev and test databases
	@brew services run $(PG_FORMULA) >/dev/null 2>&1 || true
	@for i in 1 2 3 4 5 6 7 8 9 10; do $(PG_BIN)/pg_isready -q && break; sleep 1; done
	@$(PG_BIN)/pg_isready
	@$(PG_BIN)/createdb $(DEV_DB) 2>/dev/null || true
	@$(PG_BIN)/createdb $(TEST_DB) 2>/dev/null || true

.PHONY: db-down
db-down: ## Stop Postgres
	brew services stop $(PG_FORMULA)

.PHONY: db-psql
db-psql: ## Open psql on the dev database
	$(PG_BIN)/psql $(DEV_DB)

.PHONY: db-reset
db-reset: ## Drop and recreate the dev database (deletes all local jobs)
	$(PG_BIN)/dropdb --if-exists $(DEV_DB)
	$(PG_BIN)/createdb $(DEV_DB)

# --- Server -----------------------------------------------------------------

.PHONY: server-run
server-run: ## Run the Go server on :8080 (applies migrations on start)
	cd server && go run ./cmd/server

.PHONY: server-test
server-test: ## Run the Go tests (needs `make db-up`)
	cd server && go test ./...

.PHONY: server-test-race
server-test-race: ## Go tests with the race detector, each run 3 times
	cd server && go test -race -count=3 ./...

.PHONY: server-lint
server-lint: ## go vet, and fail if any file is not gofmt-formatted
	cd server && go vet ./...
	@cd server && test -z "$$(gofmt -l .)" || (echo "not gofmt-formatted:"; gofmt -l .; exit 1)

.PHONY: server-build
server-build: ## Build the server binary into server/bin/
	cd server && go build -o bin/server ./cmd/server

# Uploads one second of silence and follows the job's event stream. Needs
# `make server-run` in another terminal. -N stops curl from buffering, so
# events print as they arrive.
.PHONY: sample-job
sample-job: ## Upload a test clip to the running server and stream its events
	@python3 -c "import wave; w = wave.open('/tmp/vt-sample.wav', 'wb'); w.setnchannels(1); w.setsampwidth(2); w.setframerate(16000); w.writeframes(bytes(32000)); w.close()"
	@url=$$(curl -sf -F audio=@/tmp/vt-sample.wav -F source_lang=en localhost:8080/v1/jobs | sed -E 's/.*"events_url":"([^"]+)".*/\1/') \
		&& echo "streaming $$url" && curl -sN "localhost:8080$$url"

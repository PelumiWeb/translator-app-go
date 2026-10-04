# Common commands for the monorepo. Run `make help` for the list.

# postgresql@16 is "keg-only" in Homebrew: its binaries are not put on PATH,
# so they are called through their full path.
PG_FORMULA := postgresql@16
PG_BIN     := $(shell brew --prefix)/opt/$(PG_FORMULA)/bin

DEV_DB  := voice_translation
TEST_DB := voice_translation_test

# The Whisper model used on the device: multilingual "base", quantised, 57 MB.
# It is downloaded, never committed. server/data/ is git-ignored, and this is
# where the server will serve models from in milestone 4.
MODEL      := ggml-base-q5_1.bin
MODEL_FILE := server/data/models/$(MODEL)
MODEL_URL  := https://huggingface.co/ggerganov/whisper.cpp/resolve/main/$(MODEL)
APP_ID     := com.example.ptranslate

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

# --- Android ----------------------------------------------------------------

.PHONY: android-build
android-build: ## Build the debug APK
	cd android && ./gradlew :app:assembleDebug

.PHONY: android-test
android-test: ## Run the Android JVM unit tests
	cd android && ./gradlew :app:testDebugUnitTest

# `adb reverse` forwards the device's localhost:8080 to this machine, so the
# same backend URL works on the emulator and on a phone connected by USB. It
# is forgotten when the device disconnects, so it is set on every install.
.PHONY: device-proxy
device-proxy: ## Let the connected device reach the local server
	adb reverse tcp:8080 tcp:8080

.PHONY: android-install
android-install: device-proxy ## Install the debug build on the connected device and open it
	cd android && ./gradlew :app:installDebug
	adb shell am start -n $(APP_ID)/.MainActivity

.PHONY: android-device-test
android-device-test: ## Run the tests that need a device (native code). Removes the app afterwards
	cd android && ./gradlew :app:connectedDebugAndroidTest

# --- Whisper model ----------------------------------------------------------

# A file target: make skips the download when the file already exists.
# -C - resumes a partial download.
$(MODEL_FILE):
	mkdir -p $(dir $(MODEL_FILE))
	curl -L --fail -C - -o $(MODEL_FILE) $(MODEL_URL)

.PHONY: model-download
model-download: $(MODEL_FILE) ## Download the Whisper model (57 MB, once)

# Until the model manager exists (milestone 4) the model is copied by hand.
# It goes to two places: /data/local/tmp for the device tests, which survive
# the app being uninstalled, and the app's own files, where the app looks.
# run-as only works for debuggable builds, and only once the app is installed.
.PHONY: android-push-model
android-push-model: $(MODEL_FILE) ## Copy the model to the connected device
	adb shell mkdir -p /data/local/tmp/ptranslate
	adb push $(MODEL_FILE) /data/local/tmp/ptranslate/$(MODEL)
	@if adb shell pm path $(APP_ID) >/dev/null 2>&1; then \
		adb shell run-as $(APP_ID) mkdir -p files/models && \
		adb shell run-as $(APP_ID) cp /data/local/tmp/ptranslate/$(MODEL) files/models/$(MODEL) && \
		echo "model copied into the app"; \
	else \
		echo "app not installed: run make android-install, then this again"; \
	fi

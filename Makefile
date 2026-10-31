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

# server/.env holds secrets, such as the voice engine's key, as NAME=value
# lines; git ignores it. Its variables are exported to the server.
SERVER_ENV := set -a; [ ! -f .env ] || . ./.env; set +a;

.PHONY: server-run
server-run: ## Run the Go server on :8080 (applies migrations on start)
	cd server && $(SERVER_ENV) go run ./cmd/server

# The two halves as separate processes. Run each in its own terminal.
.PHONY: server-run-api
server-run-api: ## Run only the HTTP API (pair with server-run-worker)
	cd server && $(SERVER_ENV) go run ./cmd/server -role api

.PHONY: server-run-worker
server-run-worker: ## Run only the job workers (pair with server-run-api)
	cd server && $(SERVER_ENV) go run ./cmd/server -role worker

.PHONY: server-run-flaky
server-run-flaky: ## Run the server with a provider that fails each job twice, to watch retries
	cd server && $(SERVER_ENV) FAKE_FAIL_ATTEMPTS=2 go run ./cmd/server

# One real request to the voice engine, with whisper.cpp's sample clip as the
# voice, then plays the result. Costs a fraction of a cent.
.PHONY: voice-check
voice-check: ## Ask Fish Audio to speak once (key from server/.env) and play what it made
	cd server && $(SERVER_ENV) FISH_LIVE_OUT=/tmp/vt-fish.wav go test ./internal/provider/fish -run TestLiveSynthesis -count=1 -v
	afplay /tmp/vt-fish.wav

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

# The emulator records silence unless told to use the Mac's microphone, and
# it forgets that every time it starts. Run this after starting the emulator.
.PHONY: emulator-mic
emulator-mic: ## Let the emulator hear the Mac's microphone (needed after every emulator start)
	adb emu avd hostmicon

.PHONY: android-install
android-install: device-proxy ## Install the debug build on the connected device and open it
	cd android && ./gradlew :app:installDebug
	adb shell am start -n $(APP_ID)/.MainActivity

.PHONY: android-device-test
android-device-test: ## Run the tests that need a device (native code). Removes the app afterwards
	cd android && ./gradlew :app:connectedDebugAndroidTest

# --- iOS --------------------------------------------------------------------

# The simulator to build and test on. Override for another one:
# `make ios-test IOS_SIM="iPhone 17"`. `xcrun simctl list devices` lists them.
IOS_PROJECT := ios/PTranslate.xcodeproj
IOS_SIM     ?= iPhone 17 Pro
IOS_DEST    := platform=iOS Simulator,name=$(IOS_SIM)

# The app links whisper.cpp as an xcframework built from the submodule. The
# script does nothing when it is already built from the submodule's commit,
# so the targets below can always depend on it.
.PHONY: ios-whisper
ios-whisper: ## Build whisper.cpp for iOS into ios/build/whisper.xcframework (once, about a minute)
	ios/scripts/build-whisper.sh

.PHONY: ios-build
ios-build: ios-whisper ## Build the iOS app for the simulator
	xcodebuild build -project $(IOS_PROJECT) -scheme PTranslate -destination '$(IOS_DEST)' -quiet

# The Whisper tests read the model from server/data/models; without it they
# are skipped, so run `make model-download` first to include them.
# -quiet prints nothing when every test passes, so the counts are read back
# from the result bundle afterwards. The exit status is xcodebuild's.
IOS_RESULTS := ios/build/test-results.xcresult

.PHONY: ios-test
ios-test: ios-whisper ## Run the iOS tests on the simulator and print the counts
	@rm -rf $(IOS_RESULTS)
	@echo "xcodebuild test on $(IOS_SIM)"
	@xcodebuild test -project $(IOS_PROJECT) -scheme PTranslate -destination '$(IOS_DEST)' -quiet \
		-resultBundlePath $(IOS_RESULTS); \
	status=$$?; \
	if [ -d $(IOS_RESULTS) ]; then \
		xcrun xcresulttool get test-results summary --path $(IOS_RESULTS) | python3 -I ios/scripts/test-summary.py; \
	fi; \
	exit $$status

# A real iPhone has no localhost to the Mac: it reaches the server by the
# Mac's Bonjour name on the same Wi-Fi. The project sets that name for phone
# builds; this target reads it afresh, in case the Mac was renamed. The phone
# must be unlocked, with Developer Mode on (Settings, Privacy & Security).
IOS_DEVICE ?= iPhone
MAC_HOST   := $(shell scutil --get LocalHostName 2>/dev/null).local

.PHONY: ios-device
ios-device: ios-whisper ## Build, install and open the app on the connected iPhone; start the server first
	xcodebuild build -project $(IOS_PROJECT) -scheme PTranslate -destination 'platform=iOS,name=$(IOS_DEVICE)' \
		-allowProvisioningUpdates -quiet BACKEND_URL=http://$(MAC_HOST):8080
	@app=$$(xcodebuild -project $(IOS_PROJECT) -scheme PTranslate -destination 'generic/platform=iOS' -showBuildSettings 2>/dev/null \
		| awk -F' = ' '/ TARGET_BUILD_DIR /{dir=$$2} / FULL_PRODUCT_NAME /{name=$$2} END{print dir "/" name}'); \
	xcrun devicectl device install app --device "$(IOS_DEVICE)" "$$app"
	xcrun devicectl device process launch --device "$(IOS_DEVICE)" pelumi.PTranslate
	@echo "The app on the phone talks to http://$(MAC_HOST):8080"

# --- Whisper model ----------------------------------------------------------

# A file target: make skips the download when the file already exists.
# -C - resumes a partial download.
$(MODEL_FILE):
	mkdir -p $(dir $(MODEL_FILE))
	curl -L --fail -C - -o $(MODEL_FILE) $(MODEL_URL)

.PHONY: model-download
model-download: $(MODEL_FILE) ## Download the Whisper model (57 MB, once)

# The app downloads its own model from the server (the Download button on
# screen). This target is for the device tests, which read the model from
# /data/local/tmp so that it survives the app being uninstalled. It also drops
# a copy into the installed app, which skips the in-app download.
# run-as only works for debuggable builds, and only once the app is installed.
.PHONY: android-push-model
android-push-model: $(MODEL_FILE) ## Copy the model to the device for the device tests
	adb shell mkdir -p /data/local/tmp/ptranslate
	adb push $(MODEL_FILE) /data/local/tmp/ptranslate/$(MODEL)
	@if adb shell pm path $(APP_ID) >/dev/null 2>&1; then \
		adb shell run-as $(APP_ID) mkdir -p files/models && \
		adb shell run-as $(APP_ID) cp /data/local/tmp/ptranslate/$(MODEL) files/models/$(MODEL) && \
		echo "model copied into the app"; \
	else \
		echo "app not installed: run make android-install, then this again"; \
	fi

# Asks the running server to "speak" a sentence in the voice of a sample, and
# plays the result. With the fake engine the answer is one beep per word.
# The voice sample here is three seconds of silence: the fake does not listen.
.PHONY: sample-speech
sample-speech: ## Ask the running server to speak a sentence (start it with VOICE_ENGINE=fake), and play the result
	@python3 -c "import wave; w = wave.open('/tmp/vt-voice.wav', 'wb'); w.setnchannels(1); w.setsampwidth(2); w.setframerate(16000); w.writeframes(bytes(96000)); w.close()"
	@resp=$$(curl -sf -F voice=@/tmp/vt-voice.wav -F lang=es -F "text=Buenos dias, como estas hoy" localhost:8080/v1/speech) \
		&& events=$$(echo "$$resp" | sed -E 's/.*"events_url":"([^"]+)".*/\1/') \
		&& audio=$$(echo "$$resp" | sed -E 's/.*"audio_url":"([^"]+)".*/\1/') \
		&& curl -sN "localhost:8080$$events" \
		&& curl -sf -o /tmp/vt-speech.wav "localhost:8080$$audio" \
		&& echo "saved /tmp/vt-speech.wav, playing it" && afplay /tmp/vt-speech.wav

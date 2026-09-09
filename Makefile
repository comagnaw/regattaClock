include version

# Repo identity is derived, not hard-coded, so a fork or repo move needs no edit
# here. MODULE is the go.mod module path - the real source of truth for the -X
# symbol prefix (if LD_IMPORTPATH drifts from the linked package path the -X
# flags silently no-op). REPO is its last element, passed as the Project
# attribute by scripts/compile.
MODULE := $(shell awk '/^module /{print $$2}' go.mod)
export REPO = $(notdir $(MODULE))
export REPO_ROOT := $(shell git rev-parse --show-toplevel)
# where we download dependencies and create artifacts
export BUILD_DIR = ${REPO_ROOT}/.build
export BIN_DIR = $(BUILD_DIR)/bin

export GIT_COMMIT = $(shell git rev-parse --short HEAD)
export CHANGE_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
export LD_IMPORTPATH = $(MODULE)/internal/version
# Canonical source location, compiled in as internal/version.RepoURL. Taken from
# the origin remote (normalised to https://host/org/repo) so it tracks a move;
# falls back to https:// + module path for a checkout with no remote.
ORIGIN_URL := $(shell git config --get remote.origin.url 2>/dev/null)
export REPO_URL := $(shell echo "$(or $(ORIGIN_URL),https://$(MODULE))" | \
	sed -E 's|^git@([^:]+):|https://\1/|; s|^ssh://git@|https://|; s|^git://|https://|; s|\.git$$||; s|/$$||')

export BINARIES = regattaClock

.PHONY: build version check test test-cover run update-deps clean

build:
	./scripts/compile

# version - print the current version string (for CI / scripts).
version:
	@echo $(VERSION)

# check - the same gate CI runs on every PR.
check:
	go build ./...
	go vet ./...
	go test ./...

test:
	go test ./... -v -covermode=atomic

test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out

run:
	go run ./cmd/regattaClock

update-deps:
	go get -u ./...
	go mod tidy

clean:
	rm -rf ${BUILD_DIR}
	rm -rf fyne-cross/
	rm -rf regattaClock.app/

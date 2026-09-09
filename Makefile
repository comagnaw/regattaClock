include version

export GH_HOST = github.com
export ORG = comagnaw
export REPO = regattaClock
export REPO_ROOT := $(shell git rev-parse --show-toplevel)
# where we download dependencies and create artifacts
export BUILD_DIR = ${REPO_ROOT}/.build
export BIN_DIR = $(BUILD_DIR)/bin

export GIT_COMMIT = $(shell git rev-parse --short HEAD)
export CHANGE_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
export LD_IMPORTPATH = ${GH_HOST}/${ORG}/${REPO}/internal/version
# Canonical source location, assembled here (not hard-coded in version.go) so a
# repo move is a one-line edit. Compiled in as internal/version.RepoURL.
export REPO_URL = https://${GH_HOST}/${ORG}/${REPO}

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

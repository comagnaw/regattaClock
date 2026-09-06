
export BUILD_DIR = .build
export BIN_DIR = $(BUILD_DIR)/bin

PHONY: *

build:
	go build -o ${BIN_DIR}/ ./...

test:
	go test ./... -v -covermode=atomic

test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out

run:
	go run ./cmd/regattaClock

# run-director - the Regatta Director window (Excel import, schedule ownership).
# Use this to set up a regatta; the timer needs a published regattaSchedule.json.
run-director:
	go run ./cmd/regattaDirector

update-deps:
	go get -u ./...
	go mod tidy

clean:
	rm -rf ${BUILD_DIR}
	rm -rf fyne-cross/
	rm -rf regattaClock.app/

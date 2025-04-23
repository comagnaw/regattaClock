
export BUILD_DIR = .build
export BIN_DIR = $(BUILD_DIR)/bin

PHONY: *

build:
	go build -o ${BIN_DIR}/ ./...

run:
	go run cmd/regattaClock/main.go

clean:
	rm -rf ${BUILD_DIR}
	rm -rf fyne-cross/

BINARY_NAME=multigravity
BUILD_DIR=bin
PREFIX ?= /usr/local
INSTALL_DIR ?= $(PREFIX)/bin

.PHONY: all build install clean test cross-compile

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/multigravity

install: build
	@mkdir -p $(DESTDIR)$(INSTALL_DIR)
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(DESTDIR)$(INSTALL_DIR)/$(BINARY_NAME)

test:
	go test -v ./...

cross-compile:
	@mkdir -p $(BUILD_DIR)/dist
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/dist/$(BINARY_NAME)-linux-amd64 ./cmd/multigravity
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/dist/$(BINARY_NAME)-linux-arm64 ./cmd/multigravity
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/dist/$(BINARY_NAME)-darwin-amd64 ./cmd/multigravity
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/dist/$(BINARY_NAME)-darwin-arm64 ./cmd/multigravity
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/multigravity

clean:
	rm -rf $(BUILD_DIR)

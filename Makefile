BINARY     := cyberlauncher
VERSION    := 0.0.1
BUILD_DIR  := ./dist
INSTALL_DIR:= $(HOME)/.local/bin

LDFLAGS    := -ldflags "-X main.version=$(VERSION) -s -w"

.PHONY: all build install clean deps run

all: build

## deps: Fetch dependencies and generate go.sum
deps:
	go mod tidy

## build: Compile the binary into ./dist/
build: deps
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) .
	@echo "✓  Built $(BUILD_DIR)/$(BINARY)"

## install: Build and install the binary to ~/.local/bin/
install: build
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "✓  Installed to $(INSTALL_DIR)/$(BINARY)"
	@echo "   Make sure $(INSTALL_DIR) is in your PATH."

## run: Build and run immediately in TUI mode
run: build
	$(BUILD_DIR)/$(BINARY)

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## help: Show this help
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'

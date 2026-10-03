PLUGIN_ID := cpa-live-voice
VERSION ?= 0.1.2

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

BIN_DIR := bin
DIST_DIR := dist

UNAME_S := $(shell uname -s)
ifeq ($(OS),Windows_NT)
  LIB_EXT := dll
else ifeq ($(UNAME_S),Darwin)
  LIB_EXT := dylib
else
  LIB_EXT := so
endif

LIB_NAME := $(PLUGIN_ID).$(LIB_EXT)
LIB_OUT := $(BIN_DIR)/$(LIB_NAME)

ARCHIVE_NAME := $(PLUGIN_ID)_$(VERSION)_$(GOOS)_$(GOARCH).zip
ARCHIVE_OUT := $(DIST_DIR)/$(ARCHIVE_NAME)

.PHONY: all test vet fmt build package check clean

all: test vet build

fmt:
	gofmt -s -w .

vet:
	go vet ./...

test:
	go test -v -race ./...

build: $(LIB_OUT)

$(LIB_OUT): $(BIN_DIR)
	go build -buildmode=c-shared -o $(LIB_OUT) .
	rm -f $(BIN_DIR)/$(PLUGIN_ID).h

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

package: build | $(DIST_DIR)
	rm -f $(ARCHIVE_OUT)
	cd $(BIN_DIR) && zip -q -9 $(abspath $(ARCHIVE_OUT)) $(LIB_NAME)
	@echo "Packaged: $(ARCHIVE_OUT)"
	cd $(DIST_DIR) && shasum -a 256 $(ARCHIVE_NAME) > checksums.txt
	@echo "Generated $(DIST_DIR)/checksums.txt"

check:
	@if [ -f "$(ARCHIVE_OUT)" ]; then \
		python3 scripts/check_package.py $(ARCHIVE_OUT) $(PLUGIN_ID) $(VERSION); \
	else \
		echo "Archive $(ARCHIVE_OUT) not found. Run 'make package' first."; \
		exit 1; \
	fi

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

APP_NAME = viri
DAEMON_NAME = virid
CLI_NAME = virictl
VERSION = 0.1.0

BUILD_DIR = build
GO_FILES = $(shell find . -name "*.go")

LDFLAGS = -ldflags "-X main.Version=$(VERSION) -s -w"

.PHONY: all build build-all clean test lint release install bench bench-ci fuzz smoke-test

all: build

build:
	@echo "Building for current platform..."
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(DAEMON_NAME) ./cmd/virid
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME) ./cmd/virictl
	@echo "Build complete: $(BUILD_DIR)/$(DAEMON_NAME) $(BUILD_DIR)/$(CLI_NAME)"

build-linux:
	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(DAEMON_NAME)-linux-amd64 ./cmd/virid
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-linux-amd64 ./cmd/virictl
	@echo "Building for Linux (arm64)..."
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(DAEMON_NAME)-linux-arm64 ./cmd/virid
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-linux-arm64 ./cmd/virictl

build-darwin:
	@echo "Building for macOS (amd64)..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(DAEMON_NAME)-darwin-amd64 ./cmd/virid
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-darwin-amd64 ./cmd/virictl
	@echo "Building for macOS (arm64)..."
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(DAEMON_NAME)-darwin-arm64 ./cmd/virid
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-darwin-arm64 ./cmd/virictl

build-windows:
	@echo "Building for Windows (amd64)..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(DAEMON_NAME)-windows-amd64.exe ./cmd/virid
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-windows-amd64.exe ./cmd/virictl

build-rpi:
	@echo "Building for Raspberry Pi (ARMv6)..."
	GOOS=linux GOARCH=arm GOARM=6 go build $(LDFLAGS) -o $(BUILD_DIR)/$(DAEMON_NAME)-linux-armv6 ./cmd/virid
	GOOS=linux GOARCH=arm GOARM=6 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-linux-armv6 ./cmd/virictl
	@echo "Building for Raspberry Pi 4 (ARM64)..."
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(DAEMON_NAME)-linux-arm64 ./cmd/virid
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-linux-arm64 ./cmd/virictl

build-all: build-linux build-darwin build-windows build-rpi
	@echo "All platforms built successfully!"

test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

lint:
	@echo "Running linter..."
	golangci-lint run ./...

bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem -count=1 -run=^$$ ./tests/benchmarks/... -timeout 10m
	go test -bench=. -benchmem -count=1 -run=^$$ ./internal/layer2/vm/... -timeout 5m

bench-ci:
	@echo "Running CI-style benchmarks (short)..."
	go test -bench=. -benchmem -count=1 -benchtime=1x -run=^$$ ./tests/benchmarks/... -timeout 5m

fuzz:
	@echo "Running fuzz tests (30s each)..."
	go test -fuzz=FuzzHasSuperMajority -fuzztime=30s ./internal/layer1/consensus/...
	go test -fuzz=FuzzSignatureVerification -fuzztime=30s ./tests/fuzz/...
	go test -fuzz=FuzzTransactionHash -fuzztime=30s ./tests/fuzz/...
	go test -fuzz=FuzzMerkleTree -fuzztime=30s ./tests/fuzz/...
	go test -fuzz=FuzzECDSASignVerify -fuzztime=30s ./tests/fuzz/...

smoke-test:
	@echo "Running Docker testnet smoke test..."
	@cd testnet && bash smoke_test.sh

clean:
	@echo "Cleaning build directory..."
	rm -rf $(BUILD_DIR)

release: clean build-all
	@echo "Creating release package..."
	mkdir -p $(BUILD_DIR)/release
	cd $(BUILD_DIR) && tar -czf release/$(APP_NAME)-$(VERSION)-linux-amd64.tar.gz $(DAEMON_NAME)-linux-amd64 $(CLI_NAME)-linux-amd64
	cd $(BUILD_DIR) && tar -czf release/$(APP_NAME)-$(VERSION)-linux-arm64.tar.gz $(DAEMON_NAME)-linux-arm64 $(CLI_NAME)-linux-arm64
	cd $(BUILD_DIR) && tar -czf release/$(APP_NAME)-$(VERSION)-linux-armv6.tar.gz $(DAEMON_NAME)-linux-armv6 $(CLI_NAME)-linux-armv6
	cd $(BUILD_DIR) && tar -czf release/$(APP_NAME)-$(VERSION)-darwin-amd64.tar.gz $(DAEMON_NAME)-darwin-amd64 $(CLI_NAME)-darwin-amd64
	cd $(BUILD_DIR) && tar -czf release/$(APP_NAME)-$(VERSION)-darwin-arm64.tar.gz $(DAEMON_NAME)-darwin-arm64 $(CLI_NAME)-darwin-arm64
	cd $(BUILD_DIR) && zip -j release/$(APP_NAME)-$(VERSION)-windows-amd64.zip $(DAEMON_NAME)-windows-amd64.exe $(CLI_NAME)-windows-amd64.exe
	cd $(BUILD_DIR)/release && sha256sum * > SHA256SUMS.txt
	@echo "Release packages created in $(BUILD_DIR)/release/"

install: build
	@echo "Installing binaries to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(DAEMON_NAME) /usr/local/bin/
	sudo cp $(BUILD_DIR)/$(CLI_NAME) /usr/local/bin/
	@echo "Installation complete!"

docker:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) -f docker/Dockerfile .

# Testnet Deployment
VALIDATORS ?= 4
CHAIN_ID ?= 1337
STAKE ?= 1000000
OUTPUT_DIR ?= testnet

.PHONY: testnet-init testnet-start testnet-stop testnet-status testnet-clean testnet-logs testnet-full

testnet-init:
	@echo "Initializing testnet with $(VALIDATORS) validators..."
	@bash deploy/testnet-init.sh \
		--validators $(VALIDATORS) \
		--chain-id $(CHAIN_ID) \
		--stake $(STAKE) \
		--output-dir $(OUTPUT_DIR) \
		--build \
		--monitoring

testnet-start:
	@echo "Starting testnet..."
	@cd $(OUTPUT_DIR) && bash start.sh

testnet-stop:
	@echo "Stopping testnet..."
	@cd $(OUTPUT_DIR) && bash stop.sh

testnet-status:
	@cd $(OUTPUT_DIR) && bash status.sh

testnet-logs:
	@cd $(OUTPUT_DIR) && docker compose logs -f

testnet-clean:
	@echo "Cleaning testnet..."
	@cd $(OUTPUT_DIR) && docker compose down -v
	@rm -rf $(OUTPUT_DIR)
	@echo "Testnet removed"

testnet-full: testnet-init testnet-start testnet-status

docker-build:
	@echo "Building Docker image for testnet..."
	docker build -t $(APP_NAME):latest .

docker-testnet: docker-build
	@echo "Starting Docker testnet..."
	@bash deploy/testnet-init.sh \
		--validators $(VALIDATORS) \
		--chain-id $(CHAIN_ID) \
		--output-dir $(OUTPUT_DIR) \
		--monitoring --explorer

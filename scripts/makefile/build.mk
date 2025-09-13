###############################################################################
###                                  Build                                  ###
###############################################################################

build-help:
	@echo "Available build commands:"
	@echo ""
	@echo "Usage:"
	@echo "  make build-[command]"
	@echo ""
	@echo "Available build subcommands:"
	@echo "  build-linux          Build babylond linux version binary"
	@echo "  build-testnet       Build babylond testnet version binary"
	@echo "  build-mocks         Build babylond mocks"
	@echo "  build-clean         Clean build artifacts"
	@echo ""

BUILD_TARGETS := build install

PACKAGES_E2E=$(shell go list ./... | grep '/e2e')

build: BUILD_ARGS=-o $(BUILDDIR)/ ## Build babylond binary
build-linux: ## Build babylond linux version binary
	GOOS=linux GOARCH=$(if $(findstring aarch64,$(shell uname -m)) || $(findstring arm64,$(shell uname -m)),arm64,amd64) LEDGER_ENABLED=false $(MAKE) build

$(BUILD_TARGETS): $(BUILDDIR)/
	go $@ -mod=readonly $(BUILD_FLAGS) $(BUILD_ARGS) ./...

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

build-testnet:
	BABYLON_BUILD_OPTIONS=testnet make build

.PHONY: build build-linux build-testnet

mockgen_cmd=go run github.com/golang/mock/mockgen@v1.6.0

mocks: $(MOCKS_DIR) ## Generate mock objects for testing
	$(mockgen_cmd) -source=x/checkpointing/types/expected_keepers.go -package mocks -destination testutil/mocks/checkpointing_expected_keepers.go
	$(mockgen_cmd) -source=x/checkpointing/keeper/bls_signer.go -package mocks -destination testutil/mocks/bls_signer.go
	$(mockgen_cmd) -source=x/zoneconcierge/types/expected_keepers.go -package types -destination x/zoneconcierge/types/mocked_keepers.go
	$(mockgen_cmd) -source=x/btcstaking/types/expected_keepers.go -package types -destination x/btcstaking/types/mocked_keepers.go
	$(mockgen_cmd) -source=x/finality/types/expected_keepers.go -package types -destination x/finality/types/mocked_keepers.go
	$(mockgen_cmd) -source=x/finality/types/hooks.go -package types -destination x/finality/types/mocked_hooks.go
	$(mockgen_cmd) -source=x/incentive/types/expected_keepers.go -package types -destination x/incentive/types/mocked_keepers.go
	$(mockgen_cmd) -source=x/incentive/types/hooks.go -package types -destination x/incentive/types/mocked_hooks.go
	$(mockgen_cmd) -source=x/costaking/types/expected_keepers.go -package types -destination x/costaking/types/mocked_keepers.go
.PHONY: mocks

$(MOCKS_DIR):
	mkdir -p $(MOCKS_DIR)

clean:
	rm -rf \
    $(BUILDDIR)/ \
    artifacts/ \
    tmp-swagger-gen/

.PHONY: clean

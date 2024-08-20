#!/usr/bin/make -f

PACKAGES_NOSIMULATION=$(shell go list ./... | grep -v '/simulation')
PACKAGES_SIMTEST=$(shell go list ./... | grep '/simulation')
COMMIT := $(shell git log -1 --format='%H')
LEDGER_ENABLED ?= true
BINDIR ?= $(GOPATH)/bin
PROJECT_NAME ?= babylon
BUILDDIR ?= $(CURDIR)/build
HTTPS_GIT := https://github.com/babylonlabs-io/babylon.git
DOCKER := $(shell which docker)
SIMAPP = ./simapp

BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT := $(shell git log -1 --format='%H')

CUR_DIR := $(shell pwd)

WASM_DIR := $(CUR_DIR)/wasmbinding/testdata

WASM_DIR_BASE_NAME := $(shell basename $(WASM_DIR))

# don't override user values
ifeq (,$(VERSION))
  # Find a name that exactly describes the current commit (e.g. a version tag)
  VERSION := $(shell git describe --exact-match 2>/dev/null)
  # if VERSION is empty, then populate it with branch's name and raw commit hash
  ifeq (,$(VERSION))
    VERSION := $(BRANCH)-$(COMMIT)
  endif
endif

export GO111MODULE = on

# process build tags

build_tags = netgo
ifeq ($(LEDGER_ENABLED),true)
  ifeq ($(OS),Windows_NT)
    GCCEXE = $(shell where gcc.exe 2> NUL)
    ifeq ($(GCCEXE),)
      $(error gcc.exe not installed for ledger support, please install or set LEDGER_ENABLED=false)
    else
      build_tags += ledger
    endif
  else
    UNAME_S = $(shell uname -s)
    ifeq ($(UNAME_S),OpenBSD)
      $(warning OpenBSD detected, disabling ledger support (https://github.com/cosmos/cosmos-sdk/issues/1988))
    else
      GCC = $(shell command -v gcc 2> /dev/null)
      ifeq ($(GCC),)
        $(error gcc not installed for ledger support, please install or set LEDGER_ENABLED=false)
      else
        build_tags += ledger
      endif
    endif
  endif
endif

ifeq (cleveldb,$(findstring cleveldb,$(BABYLON_BUILD_OPTIONS)))
  build_tags += gcc
endif

ifeq (secp,$(findstring secp,$(BABYLON_BUILD_OPTIONS)))
  build_tags += libsecp256k1_sdk
endif

whitespace :=
whitespace := $(whitespace) $(whitespace)
comma := ,
build_tags_comma_sep := $(subst $(whitespace),$(comma),$(build_tags))

# process linker flags

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=babylon \
		  -X github.com/cosmos/cosmos-sdk/version.AppName=babylond \
		  -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
		  -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
		  -X "github.com/cosmos/cosmos-sdk/version.BuildTags=$(build_tags_comma_sep)"

# DB backend selection
ifeq (cleveldb,$(findstring cleveldb,$(BABYLON_BUILD_OPTIONS)))
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=cleveldb
endif
ifeq (badgerdb,$(findstring badgerdb,$(BABYLON_BUILD_OPTIONS)))
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=badgerdb
  BUILD_TAGS += badgerdb
endif
# handle rocksdb
ifeq (rocksdb,$(findstring rocksdb,$(BABYLON_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_TAGS += rocksdb
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=rocksdb
endif
# handle boltdb
ifeq (boltdb,$(findstring boltdb,$(BABYLON_BUILD_OPTIONS)))
  BUILD_TAGS += boltdb
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=boltdb
endif

ifeq ($(LINK_STATICALLY),true)
	ldflags += -linkmode=external -extldflags "-Wl,-z,muldefs -static"
endif

ifeq (,$(findstring nostrip,$(BABYLON_BUILD_OPTIONS)))
  ldflags += -w -s
endif
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

build_tags += $(BUILD_TAGS)
build_tags := $(strip $(build_tags))

BUILD_FLAGS := -tags "$(build_tags)" -ldflags '$(ldflags)'
# check for nostrip option
ifeq (,$(findstring nostrip,$(BABYLON_BUILD_OPTIONS)))
  BUILD_FLAGS += -trimpath
endif

# Update changelog vars
ifneq (,$(SINCE_TAG))
	since_tag := --since-tag $(SINCE_TAG)
endif
ifneq (,$(UPCOMING_TAG))
	upcoming_tag := --upcoming-tag $(UPCOMING_TAG)
endif

help: ## Print this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

all: tools build lint test ## Run build, lint, and test

# The below include contains the tools and runsim targets.
# TODO: Fix following make file so it will work on linux
# include contrib/devtools/Makefile

###############################################################################
###                                  Build                                  ###
###############################################################################

BUILD_TARGETS := build install

PACKAGES_E2E=$(shell go list ./... | grep '/e2e')

build: BUILD_ARGS=-o $(BUILDDIR)/ ## Build babylond binary
build-linux: ## Build babylond linux version binary
	GOOS=linux GOARCH=$(if $(findstring aarch64,$(shell uname -m)) || $(findstring arm64,$(shell uname -m)),arm64,amd64) LEDGER_ENABLED=false $(MAKE) build

$(BUILD_TARGETS): $(BUILDDIR)/
	go $@ -mod=readonly $(BUILD_FLAGS) $(BUILD_ARGS) ./...

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

.PHONY: build build-linux

mockgen_cmd=go run github.com/golang/mock/mockgen@v1.6.0

mocks: $(MOCKS_DIR) ## Generate mock objects for testing
	$(mockgen_cmd) -source=x/checkpointing/types/expected_keepers.go -package mocks -destination testutil/mocks/checkpointing_expected_keepers.go
	$(mockgen_cmd) -source=x/checkpointing/keeper/bls_signer.go -package mocks -destination testutil/mocks/bls_signer.go
	$(mockgen_cmd) -source=x/zoneconcierge/types/expected_keepers.go -package types -destination x/zoneconcierge/types/mocked_keepers.go
	$(mockgen_cmd) -source=x/btcstaking/types/expected_keepers.go -package types -destination x/btcstaking/types/mocked_keepers.go
	$(mockgen_cmd) -source=x/finality/types/expected_keepers.go -package types -destination x/finality/types/mocked_keepers.go
	$(mockgen_cmd) -source=x/incentive/types/expected_keepers.go -package types -destination x/incentive/types/mocked_keepers.go
.PHONY: mocks

$(MOCKS_DIR):
	mkdir -p $(MOCKS_DIR)

distclean: clean tools-clean ## Remove all files generated by builds and remove all installed tools
clean: ## Remove all files generated by builds
	rm -rf \
    $(BUILDDIR)/ \
    artifacts/ \
    tmp-swagger-gen/

.PHONY: distclean clean

###############################################################################
###                          Tools & Dependencies                           ###
###############################################################################

go.sum: go.mod
	echo "Ensure dependencies have not been modified ..." >&2
	go mod verify
	go mod tidy

###############################################################################
###                              Documentation                              ###
###############################################################################

# This builds a docs site for each branch/tag in `./docs/versions`
# and copies each site to a version prefixed path. The last entry inside
# the `versions` file will be the default root index.html.
build-docs: diagrams ## Builds a docs site
	@cd client/docs && \
	while read -r branch path_prefix; do \
		(git checkout $${branch} && npm install && VUEPRESS_BASE="/$${path_prefix}/" npm run build) ; \
		mkdir -p ~/output/$${path_prefix} ; \
		cp -r .vuepress/dist/* ~/output/$${path_prefix}/ ; \
		cp ~/output/$${path_prefix}/index.html ~/output ; \
	done < versions ;
.PHONY: build-docs

###############################################################################
###                               E2E build                                 ###
###############################################################################

# Executed to build the binary for chain initialization, one of
## chain => test/e2e/initialization/chain/main.go
## node  => test/e2e/initialization/node/main.go
e2e-build-script:
	mkdir -p $(BUILDDIR)
	go build -mod=readonly $(BUILD_FLAGS) -o $(BUILDDIR)/ ./test/e2e/initialization/$(E2E_SCRIPT_NAME)

###############################################################################
###                           Tests & Simulation                            ###
###############################################################################

test: test-unit ## Run unit tests
test-all: test-unit test-ledger-mock test-race test-cover ## Run all tests

TEST_PACKAGES=./...
TEST_TARGETS := test-unit test-unit-amino test-unit-proto test-ledger-mock test-race test-ledger test-race test-cover

# Test runs-specific rules. To add a new test target, just add
# a new rule, customise ARGS or TEST_PACKAGES ad libitum, and
# append the new rule to the TEST_TARGETS list.
test-unit: ARGS=-tags='cgo ledger test_ledger_mock norace'
test-unit-amino: ARGS=-tags='ledger test_ledger_mock test_amino norace'
test-ledger: ARGS=-tags='cgo ledger norace'
test-ledger-mock: ARGS=-tags='ledger test_ledger_mock norace'
test-race: ARGS=-race -tags='cgo ledger test_ledger_mock'
test-race: TEST_PACKAGES=$(PACKAGES_NOSIMULATION)
test-cover: ARGS=-timeout=30m -coverprofile=coverage.txt -tags='norace' -covermode=atomic
$(TEST_TARGETS): run-tests

# check-* compiles and collects tests without running them
# note: go test -c doesn't support multiple packages yet (https://github.com/golang/go/issues/15513)
CHECK_TEST_TARGETS := check-test-unit check-test-unit-amino
check-test-unit: ARGS=-tags='cgo ledger test_ledger_mock norace'
check-test-unit-amino: ARGS=-tags='ledger test_ledger_mock test_amino norace'
$(CHECK_TEST_TARGETS): EXTRA_ARGS=-run=none
$(CHECK_TEST_TARGETS): run-tests

run-tests:
ifneq (,$(shell which tparse 2>/dev/null))
	go test -mod=readonly -json $(ARGS) $(EXTRA_ARGS) $(TEST_PACKAGES) | tparse
else
	go test -mod=readonly $(ARGS)  $(EXTRA_ARGS) $(TEST_PACKAGES)
endif

.PHONY: run-tests test test-all $(TEST_TARGETS)

test-e2e: build-docker-e2e test-e2e-cache

test-e2e-cache:
	go test -mod=readonly -timeout=60m -v $(PACKAGES_E2E) --tags=e2e

test-e2e-cache-ibc-transfer:
	go test -run TestIBCTranferTestSuite -mod=readonly -timeout=60m -v $(PACKAGES_E2E) --tags=e2e

test-e2e-cache-btc-timestamping:
	go test -run TestBTCTimestampingTestSuite -mod=readonly -timeout=60m -v $(PACKAGES_E2E) --tags=e2e

test-e2e-cache-btc-timestamping-phase-2-hermes:
	go test -run TestBTCTimestampingPhase2HermesTestSuite -mod=readonly -timeout=60m -v $(PACKAGES_E2E) --tags=e2e

test-e2e-cache-btc-timestamping-phase-2-rly:
	go test -run TestBTCTimestampingPhase2RlyTestSuite -mod=readonly -timeout=60m -v $(PACKAGES_E2E) --tags=e2e

test-e2e-cache-btc-staking:
	go test -run TestBTCStakingTestSuite -mod=readonly -timeout=60m -v $(PACKAGES_E2E) --tags=e2e

test-e2e-cache-upgrade-signet:
	go test -run TestSoftwareUpgradeSignetLaunchTestSuite -mod=readonly -timeout=60m -v $(PACKAGES_E2E) --tags=e2e

test-sim-nondeterminism:
	@echo "Running non-determinism test..."
	@go test -mod=readonly $(SIMAPP) -run TestAppStateDeterminism -Enabled=true \
		-NumBlocks=100 -BlockSize=200 -Commit=true -Period=0 -v -timeout 24h

test-sim-custom-genesis-fast:
	@echo "Running custom genesis simulation..."
	@echo "By default, ${HOME}/.babylond/config/genesis.json will be used."
	@go test -mod=readonly $(SIMAPP) -run TestFullAppSimulation -Genesis=${HOME}/.babylond/config/genesis.json \
		-Enabled=true -NumBlocks=100 -BlockSize=200 -Commit=true -Seed=99 -Period=5 -v -timeout 24h

test-sim-import-export: runsim
	@echo "Running application import/export simulation. This may take several minutes..."
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) -ExitOnFail 50 5 TestAppImportExport

test-sim-after-import: runsim
	@echo "Running application simulation-after-import. This may take several minutes..."
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) -ExitOnFail 50 5 TestAppSimulationAfterImport

test-sim-custom-genesis-multi-seed: runsim
	@echo "Running multi-seed custom genesis simulation..."
	@echo "By default, ${HOME}/.babylond/config/genesis.json will be used."
	@$(BINDIR)/runsim -Genesis=${HOME}/.babylond/config/genesis.json -SimAppPkg=$(SIMAPP) -ExitOnFail 400 5 TestFullAppSimulation

test-sim-multi-seed-long: runsim
	@echo "Running long multi-seed application simulation. This may take awhile!"
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) -ExitOnFail 500 50 TestFullAppSimulation

test-sim-multi-seed-short: runsim
	@echo "Running short multi-seed application simulation. This may take awhile!"
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) -ExitOnFail 50 10 TestFullAppSimulation

test-sim-benchmark-invariants:
	@echo "Running simulation invariant benchmarks..."
	@go test -mod=readonly $(SIMAPP) -benchmem -bench=BenchmarkInvariants \
	-Enabled=true -NumBlocks=1000 -BlockSize=200 \
	-Period=1 -Commit=true -Seed=57 -v -timeout 24h

.PHONY: \
test-sim-nondeterminism \
test-sim-custom-genesis-fast \
test-sim-import-export \
test-sim-after-import \
test-sim-custom-genesis-multi-seed \
test-sim-multi-seed-short \
test-sim-multi-seed-long \
test-sim-benchmark-invariants

SIM_NUM_BLOCKS ?= 500
SIM_BLOCK_SIZE ?= 200
SIM_COMMIT ?= true

test-sim-benchmark:
	@echo "Running application benchmark for numBlocks=$(SIM_NUM_BLOCKS), blockSize=$(SIM_BLOCK_SIZE). This may take awhile!"
	@go test -mod=readonly -benchmem $(SIMAPP) -bench=BenchmarkFullAppSimulation  \
		-Enabled=true -NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=$(SIM_COMMIT) -timeout 24h

test-sim-profile:
	@echo "Running application benchmark for numBlocks=$(SIM_NUM_BLOCKS), blockSize=$(SIM_BLOCK_SIZE). This may take awhile!"
	@go test -mod=readonly -benchmem $(SIMAPP) -bench=BenchmarkFullAppSimulation \
		-Enabled=true -NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=$(SIM_COMMIT) -timeout 24h -cpuprofile cpu.out -memprofile mem.out

.PHONY: test-sim-profile test-sim-benchmark


benchmark:
	@go test -mod=readonly -bench=. $(PACKAGES_NOSIMULATION)
.PHONY: benchmark

###############################################################################
###                                Linting                                  ###
###############################################################################

containerMarkdownLintImage=tmknom/markdownlint
containerMarkdownLint=babylon-markdownlint
containerMarkdownLintFix=babylon-markdownlint-fix

golangci_lint_cmd=go run github.com/golangci/golangci-lint/cmd/golangci-lint

lint: lint-go ## Run go linter
	@if docker ps -a --format '{{.Names}}' | grep -Eq "^${containerMarkdownLint}$$"; then docker start -a $(containerMarkdownLint); else docker run --name $(containerMarkdownLint) -i -v "$(CURDIR):/work" $(markdownLintImage); fi

lint-fix: ## Run go linter and fix reported issues
	$(golangci_lint_cmd) run --fix --out-format=tab --issues-exit-code=0
	@if docker ps -a --format '{{.Names}}' | grep -Eq "^${containerMarkdownLintFix}$$"; then docker start -a $(containerMarkdownLintFix); else docker run --name $(containerMarkdownLintFix) -i -v "$(CURDIR):/work" $(markdownLintImage) . --fix; fi

lint-go:
	echo $(GIT_DIFF)
	$(golangci_lint_cmd) run --out-format=tab $(GIT_DIFF)

.PHONY: lint lint-fix

format: ## Run code formater
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" -not -path "./client/docs/statik/statik.go" -not -name '*.pb.go' | xargs gofmt -w -s
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" -not -path "./client/docs/statik/statik.go" -not -name '*.pb.go' | xargs misspell -w
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" -not -path "./client/docs/statik/statik.go" -not -name '*.pb.go' | xargs goimports -w -local github.com/babylonlabs-io/babylon
.PHONY: format

###############################################################################
###                                Gosec                                    ###
###############################################################################

gosec: ## Run security checks
	$(DOCKER) run --rm -it -w /$(PROJECT_NAME)/ -v $(CURDIR):/$(PROJECT_NAME) securego/gosec -exclude-generated -exclude-dir=/$(PROJECT_NAME)/testutil -exclude-dir=/$(PROJECT_NAME)/test -conf /$(PROJECT_NAME)/gosec.json /$(PROJECT_NAME)/...

gosec-local: ## Run local security checkss
	gosec -exclude-generated -exclude-dir=$(CURDIR)/testutil -exclude-dir=$(CURDIR)/test -conf $(CURDIR)/gosec.json $(CURDIR)/...

.PHONY: gosec gosec-local

###############################################################################
###                                 Devdoc                                  ###
###############################################################################

DEVDOC_SAVE = docker commit `docker ps -a -n 1 -q` devdoc:local

devdoc-init: ## Initialize documentation
	$(DOCKER) run -it -v "$(CURDIR):/go/src/github.com/babylonlabs-io/babylon" -w "/go/src/github.com/babylonlabs-io/babylon" tendermint/devdoc echo
	# TODO make this safer
	$(call DEVDOC_SAVE)

devdoc: ## Generate documentation
	$(DOCKER) run -it -v "$(CURDIR):/go/src/github.com/babylonlabs-io/babylon" -w "/go/src/github.com/babylonlabs-io/babylon" devdoc:local bash

devdoc-save: ## Save documentation changes
	# TODO make this safer
	$(call DEVDOC_SAVE)

devdoc-clean: ## Clean up documentation artifacts
	docker rmi -f $$(docker images -f "dangling=true" -q)

devdoc-update: ## Update documentation tools
	docker pull tendermint/devdoc

.PHONY: devdoc devdoc-clean devdoc-init devdoc-save devdoc-update

###############################################################################
###                                Protobuf                                 ###
###############################################################################

protoVer=0.14.0
protoImageName=ghcr.io/cosmos/proto-builder:$(protoVer)
protoImage=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace $(protoImageName)

proto-all: proto-gen proto-swagger-gen ## Generate all protobuf related files

proto-gen: ## Generate protobuf files
	@echo "Generating Protobuf files"
	@$(protoImage) sh ./proto/scripts/protocgen.sh

proto-swagger-gen: ## Generate Swagger files from protobuf
	@echo "Generating Protobuf Swagger"
	@$(protoImage) sh ./proto/scripts/protoc-swagger-gen.sh

proto-format: ## Format protobuf files
	@$(protoImage) find ./ -name "*.proto" -exec clang-format -i {} \;

proto-lint: ## Lint protobuf files
	@$(protoImage) buf lint --error-format=json

.PHONY: proto-gen proto-swagger-gen proto-format prot-lint

###############################################################################
###                                Docker                                   ###
###############################################################################
dockerNetworkList=$($(DOCKER) network ls --filter name=bbn-testnet --format {{.ID}})

build-docker: ## Build babylond Docker image
	$(MAKE) -C contrib/images babylond

build-docker-e2e:
	$(MAKE) -C contrib/images babylond-e2e
	$(MAKE) -C contrib/images babylond-before-upgrade
	$(MAKE) -C contrib/images e2e-init-chain

build-cosmos-relayer-docker: ## Build Docker image for the Cosmos relayer
	$(MAKE) -C contrib/images cosmos-relayer

clean-docker-network:
	$(DOCKER) network rm ${dockerNetworkList}

.PHONY: build-docker build-docker-e2e build-cosmos-relayer-docker clean-docker-network

###############################################################################
###                                Localnet                                 ###
###############################################################################

init-testnet-dirs: ## Initialize directories for testnet, creates a ./.testnets directory containing configuration for 4 Babylon nodes
	# need to create the dir before hand so that the docker container has write access to the `.testnets` dir
	# regardless of the user it uses
	mkdir -p $(CURDIR)/.testnets && chmod o+w $(CURDIR)/.testnets
	$(DOCKER) run --rm -v $(CURDIR)/.testnets:/home/babylon/.testnets:Z babylonlabs-io/babylond \
			  babylond testnet init-files --v 4 -o /home/babylon/.testnets \
			  --starting-ip-address 192.168.10.2 --keyring-backend=test \
			  --chain-id chain-test --btc-confirmation-depth 2 --additional-sender-account true \
			  --epoch-interval 5

localnet-start-nodes: init-testnet-dirs ## Boot the nodes described in the docker-compose.yml file
	docker-compose up -d

localnet-start: localnet-stop build-docker localnet-start-nodes ## Run with 4 nodes, a bitcoin instance, and a vigilante instance

# localnet-stop will clean up and stop all localnets running
localnet-stop:
	rm -rf $(CURDIR)/.testnets
	docker-compose down

build-test-wasm: ## Build WASM bindings for testing
	docker run --rm -v "$(WASM_DIR)":/code \
		--mount type=volume,source="$(WASM_DIR_BASE_NAME)_cache",target=/code/target \
		--mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
		cosmwasm/rust-optimizer-arm64:0.12.13
	docker run --rm -v "$(WASM_DIR)":/code \
		--mount type=volume,source="$(WASM_DIR_BASE_NAME)_cache",target=/code/target \
		--mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
		cosmwasm/rust-optimizer:0.12.13

.PHONY: \
init-testnet-dirs \
localnet-start-nodes \
localnet-start \
localnet-stop

.PHONY: diagrams
diagrams: ## Generate diagrams for documentation
	$(MAKE) -C client/docs/diagrams

.PHONY: update-changelog
update-changelog: ## Update the project changelog
	@echo ./scripts/update_changelog.sh $(since_tag) $(upcoming_tag)
	./scripts/update_changelog.sh $(since_tag) $(upcoming_tag)

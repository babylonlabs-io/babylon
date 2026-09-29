###############################################################################
###                                Release                                  ###
###############################################################################

release-help:
	@echo "Available proto commands:"
	@echo ""
	@echo "Usage:"
	@echo "  make release-[command]"
	@echo ""
	@echo "Available release subcommands:"
	@echo "  release-dry-run       Dry run the release process"
	@echo "  release-snapshot      Release a snapshot version"
	@echo "  release               Release a version"
	@echo ""

# The below is adapted from https://github.com/osmosis-labs/osmosis/blob/main/Makefile
# Go 1.25, pinned by digest — keep in sync with .github/workflows/release.yml.
# The tag alone is mutable and a Go >= 1.26 image does not build this branch's
# github.com/bytedance/sonic v1.14.2 ("undefined: GoMapIterator").
GORELEASER_IMAGE := ghcr.io/goreleaser/goreleaser-cross:v1.25.9@sha256:130e6870fec3c6a243746f092017e6b85e03a6ed865e155f5d1bce1a855d93f4
COSMWASM_VERSION := $(shell go list -m github.com/CosmWasm/wasmvm/v2 | sed 's/.* //')

release-dry-run:
	docker run \
		--rm \
		-e COSMWASM_VERSION=$(COSMWASM_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/babylon \
		-w /go/src/babylon \
		$(GORELEASER_IMAGE) \
		release \
		--clean \
		--skip=publish

release-snapshot:
	docker run \
		--rm \
		-e COSMWASM_VERSION=$(COSMWASM_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/babylon \
		-w /go/src/babylon \
		$(GORELEASER_IMAGE) \
		release \
		--clean \
		--snapshot \
		--skip=publish,validate

# NOTE: By default, the CI will handle the release process.
# this is for manually releasing.
ifdef GITHUB_TOKEN
release:
	docker run \
		--rm \
		-e GITHUB_TOKEN=$(GITHUB_TOKEN) \
		-e COSMWASM_VERSION=$(COSMWASM_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/babylon \
		-w /go/src/babylon \
		$(GORELEASER_IMAGE) \
		release \
		--clean
else
release:
	@echo "Error: GITHUB_TOKEN is not defined. Please define it before running 'make release'."
endif

.PHONY: release-dry-run release-snapshot release

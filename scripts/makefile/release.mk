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
# Go 1.26.10 (crypto/x509 fix), pinned by digest — keep in sync with .github/workflows/release.yml.
# The tag alone is mutable; sonic >= v1.15.0 is required from Go 1.26 on ("undefined: GoMapIterator").
GORELEASER_IMAGE := ghcr.io/goreleaser/goreleaser-cross:v1.26.10@sha256:4ddb0de02f2eaac64d2b33e16d1d702d800db6a110154f8768f283858fb26c02
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

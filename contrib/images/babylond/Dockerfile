FROM golang:1.24-alpine AS build-env

# Customize to your build env

# TARGETPLATFORM should be one of linux/amd64 or linux/arm64.
ARG TARGETPLATFORM="linux/amd64"
# Version to build. Default is empty
ARG VERSION
ARG BABYLON_BUILD_OPTIONS=""
ARG COSMOS_BUILD_OPTIONS=""

# Use muslc for static libs
ARG BUILD_TAGS="muslc"
ARG LEDGER_ENABLED="false"


# Install cli tools for building and final image
# hadolint ignore=DL3018
RUN apk add --no-cache make git bash gcc linux-headers eudev-dev ncurses-dev openssh curl jq musl-dev

# Build
WORKDIR /go/src/github.com/babylonlabs-io/babylon
# First cache dependencies
COPY go.mod go.sum /go/src/github.com/babylonlabs-io/babylon/
RUN go mod download
# Then copy everything else
COPY ./ /go/src/github.com/babylonlabs-io/babylon/
# If version is set, then checkout this version
RUN if [ -n "${VERSION}" ]; then \
    git fetch origin tag ${VERSION} --no-tags ; \
    git checkout -f ${VERSION}; \
    fi

# Cosmwasm - Download correct libwasmvm version
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]
RUN WASMVM_VERSION=$(go list -m github.com/CosmWasm/wasmvm/v2 | cut -d ' ' -f 2) && \
    wget -q https://github.com/CosmWasm/wasmvm/releases/download/"$WASMVM_VERSION"/libwasmvm_muslc."$(uname -m)".a \
    -O /lib/libwasmvm_muslc."$(uname -m)".a && \
    # verify checksum
    wget -q https://github.com/CosmWasm/wasmvm/releases/download/"$WASMVM_VERSION"/checksums.txt -O /tmp/checksums.txt && \
    sha256sum /lib/libwasmvm_muslc."$(uname -m)".a | grep "$(cat /tmp/checksums.txt | grep libwasmvm_muslc."$(uname -m)" | cut -d ' ' -f 1)"

RUN LEDGER_ENABLED=$LEDGER_ENABLED \
    BABYLON_BUILD_OPTIONS=$BABYLON_BUILD_OPTIONS \
    COSMOS_BUILD_OPTIONS=$COSMOS_BUILD_OPTIONS \
    BUILD_TAGS=$BUILD_TAGS \
    LINK_STATICALLY=true \
    make build

FROM alpine:3.21 AS run
# Create a user
RUN addgroup --gid 1137 -S babylon && adduser --uid 1137 -S babylon -G babylon
RUN apk add --no-cache bash=5.2.37-r0 curl=8.14.1-r2 jq=1.7.1-r0

# Label should match your github repo
ARG VERSION
LABEL org.opencontainers.image.source="https://github.com/babylonlabs-io/babylond:${VERSION}"

# Install Libraries
# COPY --from=build-env /usr/lib/libgcc_s.so.1 /lib/
# COPY --from=build-env /lib/ld-musl*.so.1* /lib

COPY --from=build-env /go/src/github.com/babylonlabs-io/babylon/build/babylond /bin/babylond

# Set home directory and user
WORKDIR /home/babylon
RUN chown -R babylon /home/babylon
USER babylon

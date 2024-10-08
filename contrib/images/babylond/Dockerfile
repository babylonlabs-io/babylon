FROM golang:1.21 AS build-env

# Version to build. Default is empty
ARG VERSION
ARG BABYLON_BUILD_OPTIONS=""
ARG LEDGER_ENABLED="false"
ARG COSMOS_BUILD_OPTIONS=""

WORKDIR /go/src/github.com/babylonlabs-io/babylon
COPY ./ /go/src/github.com/babylonlabs-io/babylon/

# Handle if version is set
RUN if [ -n "${VERSION}" ]; then \
        git fetch origin tag ${VERSION} --no-tags; \
        git checkout -f ${VERSION}; \
    fi

# Cache mod dependencies
RUN go mod download

RUN LEDGER_ENABLED=$LEDGER_ENABLED \
    BABYLON_BUILD_OPTIONS=$BABYLON_BUILD_OPTIONS \
    COSMOS_BUILD_OPTIONS=$COSMOS_BUILD_OPTIONS \
    LINK_STATICALLY=false \
    make build

FROM debian:bookworm-slim AS wasm-link

ARG VERSION

# Create a user
RUN addgroup --gid 1137 --system babylon && adduser --uid 1137 --gid 1137 --system --home /home/babylon babylon

RUN apt-get update && apt-get install -y curl wget bash jq

# Label should match your github repo
LABEL org.opencontainers.image.source="https://github.com/babylonlabs-io/babylond:${VERSION}"

# Install libraries
# Cosmwasm - Download correct libwasmvm version
COPY --from=build-env /go/src/github.com/babylonlabs-io/babylon/go.mod /tmp
RUN WASMVM_VERSION=$(grep github.com/CosmWasm/wasmvm /tmp/go.mod | cut -d' ' -f2) && \
    wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/libwasmvm.$(uname -m).so \
        -O /lib/libwasmvm.$(uname -m).so && \
    # verify checksum
    wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/checksums.txt -O /tmp/checksums.txt && \
    sha256sum /lib/libwasmvm.$(uname -m).so | grep $(cat /tmp/checksums.txt | grep libwasmvm.$(uname -m) | cut -d ' ' -f 1)
RUN rm -f /tmp/go.mod

COPY --from=build-env /go/src/github.com/babylonlabs-io/babylon/build/babylond /bin/babylond

# Set home directory and user
WORKDIR /home/babylon
RUN chown -R babylon /home/babylon
RUN chmod g+s /home/babylon
USER babylon

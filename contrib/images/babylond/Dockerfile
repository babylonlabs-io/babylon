FROM golang:1.21 as go-bbn

# Version to build. Default is empty
ARG VERSION

WORKDIR /go/src/github.com/babylonlabs-io/babylon
COPY ./ /go/src/github.com/babylonlabs-io/babylon/

# Handle if version is set
RUN if [ -n "${VERSION}" ]; then \
        git fetch origin tag ${VERSION} --no-tags; \
        git checkout -f ${VERSION}; \
    fi

FROM go-bbn as go-bbn-dep

# Cache mod dependencies
RUN go mod download

FROM go-bbn-dep as build-env

ARG BUILD_TAGS=""
ARG LEDGER_ENABLED="false"
ARG COSMOS_BUILD_OPTIONS=""

RUN LEDGER_ENABLED=$LEDGER_ENABLED \
    BUILD_TAGS=$BUILD_TAGS \
    COSMOS_BUILD_OPTIONS=$COSMOS_BUILD_OPTIONS \
    LINK_STATICALLY=false \
    make build

FROM debian:bookworm-slim AS wasm-link
ARG WASMVM_VERSION="v2.0.1"

RUN apt-get update && apt-get install -y wget bash

# Label should match your github repo
LABEL org.opencontainers.image.source="https://github.com/babylonlabs-io/babylond:${VERSION}"

# Cosmwasm - Download correct libwasmvm version
RUN wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/libwasmvm.x86_64.so \
        -O /lib/libwasmvm.x86_64.so

FROM wasm-link as final
COPY --from=build-env /go/src/github.com/babylonlabs-io/babylon/build/babylond /bin/babylond

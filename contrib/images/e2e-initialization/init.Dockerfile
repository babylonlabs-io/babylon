FROM golang:1.23 AS build-env

ARG E2E_SCRIPT_NAME
# Version to build. Default is empty
ARG VERSION

# Copy All
WORKDIR /go/src/github.com/babylonlabs-io/babylon
COPY ./ /go/src/github.com/babylonlabs-io/babylon/

# Handle if version is set
RUN if [ -n "${VERSION}" ]; then \
    git fetch origin tag ${VERSION} --no-tags ; \
    git checkout -f ${VERSION}; \
    fi

RUN LEDGER_ENABLED=false LINK_STATICALLY=false E2E_SCRIPT_NAME=${E2E_SCRIPT_NAME} make e2e-build-script

FROM debian:bookworm-slim AS wasm-link

ARG VERSION

# Create a user
RUN addgroup --gid 1137 --system babylon && adduser --uid 1137 --gid 1137 --system --home /home/babylon babylon
RUN apt-get update && apt-get install -y bash curl jq wget

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

# Args only last for a single build stage - renew
ARG E2E_SCRIPT_NAME

COPY --from=build-env /go/src/github.com/babylonlabs-io/babylon/build/${E2E_SCRIPT_NAME} /bin/${E2E_SCRIPT_NAME}
# Docker ARGs are not expanded in ENTRYPOINT in the exec mode. At the same time,
# it is impossible to add CMD arguments when running a container in the shell mode.
# As a workaround, we create the entrypoint.sh script to bypass these issues.
RUN echo "#!/bin/bash\n${E2E_SCRIPT_NAME} \"\$@\"" >> entrypoint.sh && chmod +x entrypoint.sh
ENTRYPOINT ["./entrypoint.sh"]
FROM golang:1.21 as build-env

ARG E2E_SCRIPT_NAME

# Copy All
WORKDIR /go/src/github.com/babylonlabs-io/babylon
COPY ./ /go/src/github.com/babylonlabs-io/babylon/

RUN LEDGER_ENABLED=false LINK_STATICALLY=false E2E_SCRIPT_NAME=${E2E_SCRIPT_NAME} make e2e-build-script

FROM debian:bookworm-slim AS wasm-link
ARG WASMVM_VERSION="v2.0.1"

RUN apt-get update && apt-get install -y wget bash

# Label should match your github repo
LABEL org.opencontainers.image.source="https://github.com/babylonlabs-io/babylond:${VERSION}"

# Cosmwasm - Download correct libwasmvm version
RUN wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/libwasmvm.x86_64.so \
        -O /lib/libwasmvm.x86_64.so

# Args only last for a single build stage - renew
ARG E2E_SCRIPT_NAME

COPY --from=build-env /go/src/github.com/babylonlabs-io/babylon/build/${E2E_SCRIPT_NAME} /bin/${E2E_SCRIPT_NAME}

# Docker ARGs are not expanded in ENTRYPOINT in the exec mode. At the same time,
# it is impossible to add CMD arguments when running a container in the shell mode.
# As a workaround, we create the entrypoint.sh script to bypass these issues.
RUN echo "#!/bin/bash\n${E2E_SCRIPT_NAME} \"\$@\"" >> entrypoint.sh && chmod +x entrypoint.sh

ENTRYPOINT ["./entrypoint.sh"]
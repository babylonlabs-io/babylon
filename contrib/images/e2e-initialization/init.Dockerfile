FROM golang:1.21 as build-env

ARG E2E_SCRIPT_NAME

# Install cli tools for building and final image
RUN apt-get update && apt-get install -y make git bash gcc curl jq

WORKDIR /go/src/github.com/babylonlabs-io/babylon

# First cache dependencies
COPY go.mod go.sum /go/src/github.com/babylonlabs-io/babylon/
RUN go mod download

# Copy everything else
COPY ./ /go/src/github.com/babylonlabs-io/babylon/

RUN LEDGER_ENABLED=false LINK_STATICALLY=false E2E_SCRIPT_NAME=${E2E_SCRIPT_NAME} make e2e-build-script

FROM debian:bookworm-slim AS run

# Create a user
RUN addgroup --gid 1137 --system babylon && adduser --uid 1137 --gid 1137 --system --home /home/babylon babylon
RUN apt-get update && apt-get install -y bash curl jq wget

COPY --from=build-env /go/src/github.com/babylonlabs-io/babylon/go.mod /tmp
RUN WASMVM_VERSION=$(grep github.com/CosmWasm/wasmvm /tmp/go.mod | cut -d' ' -f2) && \
    wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/libwasmvm.$(uname -m).so \
        -O /lib/libwasmvm.$(uname -m).so && \
    # verify checksum
    wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/checksums.txt -O /tmp/checksums.txt && \
    sha256sum /lib/libwasmvm.$(uname -m).so | grep $(cat /tmp/checksums.txt | grep libwasmvm.$(uname -m) | cut -d ' ' -f 1)

# Args only last for a single build stage - renew
ARG E2E_SCRIPT_NAME

COPY --from=build-env /go/src/github.com/babylonlabs-io/babylon/build/${E2E_SCRIPT_NAME} /bin/${E2E_SCRIPT_NAME}

# Docker ARGs are not expanded in ENTRYPOINT in the exec mode. At the same time,
# it is impossible to add CMD arguments when running a container in the shell mode.
# As a workaround, we create the entrypoint.sh script to bypass these issues.
RUN echo "#!/bin/bash\n${E2E_SCRIPT_NAME} \"\$@\"" >> entrypoint.sh && chmod +x entrypoint.sh

ENTRYPOINT ["./entrypoint.sh"]
FROM debian:bookworm-slim AS build-env

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates=20230311+deb12u1 \
    gcc=4:12.2.0-3 \
    git=1:2.39.5-0+deb12u3 \
    jq=1.6-2.1+deb12u1 \
    libc6-dev=2.36-9+deb12u13 \
    make=4.3-4.1 \
    wget=1.21.3-1+deb12u1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /work

ARG TARGETARCH

# Download and install Go
ENV GOLANG_VERSION=1.23.1
RUN echo "Target arch: ${TARGETARCH}" && \
    echo "Go version: ${GOLANG_VERSION}" && \
    wget -q https://golang.org/dl/go${GOLANG_VERSION}.linux-${TARGETARCH}.tar.gz && \
    tar -C /usr/local -xzf go${GOLANG_VERSION}.linux-${TARGETARCH}.tar.gz && \
    rm go${GOLANG_VERSION}.linux-${TARGETARCH}.tar.gz
# Set Go environment variables
ENV PATH=/usr/local/go/bin:$PATH
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH

WORKDIR /work

ENV GO111MODULE=on
ENV RELAYER_TAG=main
# ENV RELAYER_TAG v2.4.2

# Install the relayer
RUN git clone https://github.com/cosmos/relayer.git
WORKDIR /work/relayer
RUN git fetch origin && git checkout ${RELAYER_TAG} && make install
WORKDIR /work

FROM debian:bookworm-slim AS run
# Create a user
RUN addgroup --system rly && adduser --system --gid 101 rly

COPY --from=build-env /go/bin/rly /usr/bin/rly

# Set home directory and user
WORKDIR /home/rly
RUN chown -R rly /home/rly
USER rly

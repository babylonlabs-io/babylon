RELAYER_TAG := $(shell grep '^ENV RELAYER_TAG' cosmos-relayer/Dockerfile | cut -f3 -d\ )
BABYLON_FULL_PATH := $(shell git rev-parse --show-toplevel)
BABYLON_VERSION_BEFORE_UPGRADE ?= v0.9.3

all: babylond cosmos-relayer

babylond:
	docker build --tag babylonlabs-io/babylond -f babylond/Dockerfile ${BABYLON_FULL_PATH}

babylond-e2e:
	docker build --tag babylonlabs-io/babylond -f babylond/Dockerfile ${BABYLON_FULL_PATH} \
		--build-arg BUILD_TAGS="e2e"

## TODO: once release docker public versions for tags, remove this!
babylond-before-upgrade:
	docker build --tag babylonlabs-io/babylond-before-upgrade -f babylond/Dockerfile \
    --build-arg VERSION="${BABYLON_VERSION_BEFORE_UPGRADE}" ${BABYLON_FULL_PATH}

babylond-rmi:
	docker rmi babylonlabs-io/babylond --force 2>/dev/null; true

babylond-rmi-upgrade:
	docker rmi babylonlabs-io/babylond-before-upgrade --force 2>/dev/null; true

e2e-init-chain-rmi:
	docker rmi babylonlabs-io/babylond-e2e-init-chain --force 2>/dev/null; true

e2e-init-chain:
	@DOCKER_BUILDKIT=1 docker build -t babylonlabs-io/babylond-e2e-init-chain --build-arg E2E_SCRIPT_NAME=chain --platform=linux/x86_64 \
		-f e2e-initialization/init.Dockerfile --build-arg VERSION="${BABYLON_VERSION_BEFORE_UPGRADE}" ${BABYLON_FULL_PATH}

e2e-init-chain-rmi:
	docker rmi babylonlabs-io/babylond-e2e-init-chain 2>/dev/null; true

cosmos-relayer: cosmos-relayer-rmi
	docker build --tag babylonlabs-io/cosmos-relayer:${RELAYER_TAG} -f cosmos-relayer/Dockerfile \
				${BABYLON_FULL_PATH}/contrib/images/cosmos-relayer
	docker tag babylonlabs-io/cosmos-relayer:${RELAYER_TAG} babylonlabs-io/cosmos-relayer:latest

cosmos-relayer-rmi:
	docker rmi babylonlabs-io/cosmos-relayer 2>/dev/null; true

.PHONY: all babylond babylond-before-upgrade cosmos-relayer e2e-init-chain babylond-rmi babylond-rmi-upgrade cosmos-relayer-rmi

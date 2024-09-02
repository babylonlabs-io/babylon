RELAYER_TAG := $(shell grep '^ENV RELAYER_TAG' cosmos-relayer/Dockerfile | cut -f3 -d\ )
BABYLON_FULL_PATH := $(shell git rev-parse --show-toplevel)

all: babylond cosmos-relayer

babylond: babylond-rmi
	docker build --tag babylonlabs-io/babylond -f babylond/Dockerfile ${BABYLON_FULL_PATH}

babylond-e2e:
	docker build --tag babylonlabs-io/babylond -f babylond/Dockerfile ${BABYLON_FULL_PATH} \
		--build-arg BUILD_TAGS="e2e"

babylond-rmi:
	docker rmi babylonlabs-io/babylond 2>/dev/null; true

e2e-init-chain-rmi:
	docker rmi babylonlabs-io/babylond-e2e-init-chain --force 2>/dev/null; true

e2e-init-chain:
	docker build -t babylonlabs-io/babylond-e2e-init-chain --build-arg E2E_SCRIPT_NAME=chain --platform=linux/x86_64 \
		-f e2e-initialization/init.Dockerfile ${BABYLON_FULL_PATH}

cosmos-relayer: cosmos-relayer-rmi
	docker build --tag babylonlabs-io/cosmos-relayer:${RELAYER_TAG} -f cosmos-relayer/Dockerfile \
		${BABYLON_FULL_PATH}/contrib/images/cosmos-relayer
	docker tag babylonlabs-io/cosmos-relayer:${RELAYER_TAG} babylonlabs-io/cosmos-relayer:latest

cosmos-relayer-rmi:
	docker rmi babylonlabs-io/cosmos-relayer 2>/dev/null; true

.PHONY: all babylond cosmos-relayer babylond-e2e e2e-init-chain e2e-init-chain-rmi babylond-rmi cosmos-relayer-rmi

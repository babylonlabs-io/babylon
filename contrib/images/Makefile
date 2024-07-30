RELAYER_TAG := $(shell grep '^ENV RELAYER_TAG' cosmos-relayer/Dockerfile | cut -f3 -d\ )

all: babylond cosmos-relayer

babylond: babylond-rmi
	docker build --tag babylonlabs-io/babylond -f babylond/Dockerfile \
    		$(shell git rev-parse --show-toplevel)

babylond-rmi:
	docker rmi babylonlabs-io/babylond 2>/dev/null; true

cosmos-relayer: cosmos-relayer-rmi
	docker build --tag babylonlabs-io/cosmos-relayer:${RELAYER_TAG} -f cosmos-relayer/Dockerfile \
				$(shell git rev-parse --show-toplevel)/contrib/images/cosmos-relayer
	docker tag babylonlabs-io/cosmos-relayer:${RELAYER_TAG} babylonlabs-io/cosmos-relayer:latest

cosmos-relayer-rmi:
	docker rmi babylonlabs-io/cosmos-relayer 2>/dev/null; true

.PHONY: all babylond cosmos-relayer babylond-rmi cosmos-relayer-rmi

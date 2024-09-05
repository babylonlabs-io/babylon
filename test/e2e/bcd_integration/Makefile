DOCKER := $(shell which docker)
GIT_TOPLEVEL := $(shell git rev-parse --show-toplevel)

.PHONY: build-ibcsim-bcd build-babylond build-integration-test start-integration-test stop-integration-test

build-ibcsim-bcd:
	cd $(GIT_TOPLEVEL)/test/e2e && \
	$(DOCKER) build --no-cache --tag babylonlabs-io/ibcsim-bcd -f consumer/Dockerfile .

build-babylond:
	$(MAKE) -C $(GIT_TOPLEVEL)/contrib/images babylond

build-integration-test: build-babylond build-ibcsim-bcd

start-integration-test: stop-integration-test
	./pre-deployment.sh
	$(DOCKER) compose -f docker-compose.yml up -d

stop-integration-test:
	$(DOCKER) compose -f docker-compose.yml down
	rm -rf $(CURDIR)/.testnets

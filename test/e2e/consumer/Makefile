DOCKER := $(shell which docker)
GIT_TOPLEVEL := $(shell git rev-parse --show-toplevel)

build-ibcsim-bcd:
	docker build --tag babylonlabs-io/ibcsim-bcd -f Dockerfile \
		$(GIT_TOPLEVEL)/test/e2e/consumer

build-babylond:
	$(MAKE) -C $(GIT_TOPLEVEL)/contrib/images babylond

build-integration-test: build-babylond build-ibcsim-bcd

start-integration-test: stop-integration-test build-integration-test
	./pre-deployment.sh
	$(DOCKER) compose -f docker-compose.yml up -d

stop-integration-test:
	$(DOCKER) compose -f docker-compose.yml down
	rm -rf $(CURDIR)/.testnets

DOCKER := $(shell which docker)
GIT_TOPLEVEL := $(shell git rev-parse --show-toplevel)

build-ibcsim-bcd:
	docker build --secret id=sshKey,src=${BBN_PRIV_DEPLOY_KEY} \
		--tag babylonlabs-io/ibcsim-bcd -f Dockerfile \
		$(shell git rev-parse --show-toplevel)/test/e2e/consumer

build-babylond:
	$(MAKE) -C $(GIT_TOPLEVEL)/contrib/images babylond

build-integration-test: build-babylond build-ibcsim-bcd

start-integration-test: stop-integration-test # build-integration-test
	./pre-deployment.sh
	$(DOCKER) compose -f docker-compose.yml up -d

stop-integration-test:
	$(DOCKER) compose -f docker-compose.yml down
	rm -rf $(CURDIR)/.testnets

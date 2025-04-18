###############################################################################
###                                Gosec                                    ###
###############################################################################

gosec:
	@echo "Available security commands:"
	@echo ""
	@echo "Usage:"
	@echo "  make gosec-[command]"
	@echo ""
	@echo "Available security subcommands:"
	@echo "  gosec-docker       Run security checks using Docker"
	@echo "  gosec-local        Run local security checks"
	@echo ""

gosec-docker:
	$(DOCKER) run --rm -it -w /$(PROJECT_NAME)/ -v $(CURDIR):/$(PROJECT_NAME) securego/gosec -exclude-generated -exclude-dir=/$(PROJECT_NAME)/testutil -exclude-dir=/$(PROJECT_NAME)/test -conf /$(PROJECT_NAME)/gosec.json /$(PROJECT_NAME)/...

gosec-local:
	gosec -exclude-generated -exclude-dir=$(CURDIR)/testutil -exclude-dir=$(CURDIR)/test -conf $(CURDIR)/gosec.json $(CURDIR)/...

.PHONY: gosec-menu gosec-docker gosec-local

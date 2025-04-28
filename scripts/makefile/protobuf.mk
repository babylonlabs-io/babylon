
###############################################################################
###                                Protobuf                                 ###
###############################################################################

protoVer=0.14.0
protoImageName=ghcr.io/cosmos/proto-builder:$(protoVer)
protoImage=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace $(protoImageName)

proto-help:
	@echo "Available proto commands:"
	@echo ""
	@echo "Usage:"
	@echo "  make proto-[command]"
	@echo ""
	@echo "Available proto subcommands:"
	@echo "  proto-all            Run all linters with auto-fix"
	@echo "  proto-gen            Generate protobuf files"
	@echo "  proto-swagger-gen    Generate Swagger files from protobuf"
	@echo "  proto-format         Format Proto Files"
	@echo "  proto-lint           Lint protobuf files"
	@echo ""

proto-all: proto-gen proto-swagger-gen

proto-gen: proto-lint
	@echo "Generating Protobuf files..."
	@$(protoImage) sh ./proto/scripts/protocgen.sh

proto-swagger-gen:
	@echo "Generating Protobuf Swagger"
	@$(protoImage) sh ./proto/scripts/protoc-swagger-gen.sh

proto-format:
	@$(protoImage) find ./ -name "*.proto" -exec clang-format -i {} \;

proto-lint:
	@$(protoImage) buf lint --error-format=json

.PHONY: proto proto-gen proto-swagger-gen proto-format proto-lint
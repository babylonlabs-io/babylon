###############################################################################
###                                Linting                                  ###
###############################################################################

golangci_lint_cmd=golangci-lint

lint:
	@echo "Available linting commands:"
	@echo ""
	@echo "Usage:"
	@echo "  make lint-[command]"
	@echo ""
	@echo "Available linting subcommands:"
	@echo "  lint-all-fix        Run all linters with auto-fix"
	@echo "  lint-go             Run Go linter"
	@echo "  lint-go-fix         Fix Go linter issues"
	@echo "  lint-markdown       Run Markdown linter"
	@echo "  lint-markdown-fix   Run Markdown linter with auto-fix"
	@echo "  lint-typo           Show all of the typos"
	@echo "  lint-typo-fix       Fix all of the typos"
	@echo ""

lint-go:
	@echo "Running Go linter..."
	@$(golangci_lint_cmd) run --out-format=tab

lint-go-fix:
	@echo "Running Go linter..."
	@$(golangci_lint_cmd) run --out-format=tab --fix

lint-markdown:
	@echo "Running Markdown linter..."
	@docker run -v $(PWD):/workdir ghcr.io/igorshubovych/markdownlint-cli:latest "**/*.md"

lint-markdown-fix:
	@echo "Running Markdown linter with auto-fix..."
	@docker run -v $(PWD):/workdir ghcr.io/igorshubovych/markdownlint-cli:latest "**/*.md" --fix

lint-typo:
	@echo "Running Typo linter..."
	@codespell

lint-typo-fix:
	@echo "Running Typo linter..."
	@codespell -w

lint-all-fix:
	@echo "Running all linters..."
	@docker run -v $(PWD):/workdir ghcr.io/igorshubovych/markdownlint-cli:latest "**/*.md" --fix
	@codespell -w
	@$(golangci_lint_cmd) run --out-format=tab --fix

.PHONY: lint-go lint-go-fix lint-markdown lint-markdown-fix lint  lint-typo lint-typo-fix lint-all-fix

format: ## Run code formatter
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" -not -path "./client/docs/statik/statik.go" -not -name '*.pb.go' | xargs gofmt -w -s
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" -not -path "./client/docs/statik/statik.go" -not -name '*.pb.go' | xargs misspell -w
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" -not -path "./client/docs/statik/statik.go" -not -name '*.pb.go' | xargs goimports -w -local github.com/babylonlabs-io/babylon

.PHONY: format

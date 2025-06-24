# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Binary name
BINARY_NAME=trazr-gen

# Main package path
MAIN_PACKAGE=./cmd/trazr-gen

# Build directory
BUILD_DIR=./build

# Source files
SRC=$(shell find . -name "*.go")

# Test coverage output
COVERAGE_OUTPUT=coverage.out

.PHONY: all build clean test coverage lint deps tidy run help integration-coverage docker-build docker-run integration-test full-coverage codeql-db codeql-analyze codeql tag-major tag-minor tag-patch _tag

all: build

build: ## Build the binary
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

clean: ## Remove build artifacts
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_OUTPUT)

test: ## Run tests
	$(GOTEST) -v ./...

coverage: ## Run tests with coverage
	@mkdir -p testdata
	$(GOTEST) -v -coverprofile=testdata/coverage.out ./...
	$(GOCMD) tool cover -html=testdata/coverage.out

lint: ## Run linter
	$(GOLINT) run

deps: ## Download dependencies
	$(GOGET) -v -t -d ./...

tidy: ## Tidy and verify dependencies
	$(GOMOD) tidy
	$(GOMOD) verify

run: build ## Run the application
	$(BUILD_DIR)/$(BINARY_NAME)

docker-build: ## Build Docker image
	docker build -t $(BINARY_NAME) .

docker-run: ## Run Docker container
	docker run --rm $(BINARY_NAME)

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

e2e-coverage: ## Run all tests (including integration) with coverage and print total coverage
	$(GOTEST) -coverprofile=$(COVERAGE_OUTPUT) ./...
	@echo "\nTotal coverage:" && $(GOCMD) tool cover -func=$(COVERAGE_OUTPUT) | grep total:

e2e-test: ## Run E2E/integration tests in internal/e2etest module
	cd internal/e2etest && go test -v ./...

# Note: Go cannot natively combine coverage profiles across modules. This target runs both and prints both coverages.
full-coverage: ## Run unit and E2E tests, print both coverages
	@mkdir -p testdata
	$(GOTEST) -coverprofile=testdata/coverage.out ./...
	@echo "\nUnit test coverage:" && $(GOCMD) tool cover -func=testdata/coverage.out | grep total:
	cd internal/e2etest && go test -coverprofile=../testdata/coverage-e2e.out ./...
	@echo "\nE2E test coverage:" && $(GOCMD) tool cover -func=internal/common/testdata/coverage-e2e.out | grep total:

codeql-db: ## Create CodeQL database for Go
	codeql database create codeql-db --language=go --source-root=. --overwrite

codeql-analyze: ## Run CodeQL analysis and output SARIF
	codeql database analyze codeql-db codeql/go-queries@1.2.1 --format=sarifv2.1.0 --output=codeql-results.sarif

codeql: codeql-db codeql-analyze ## Run full CodeQL scan (create DB and analyze)

# Usage: make _tag PART=major|minor|patch
_tag:
	@set -e; \
	latest_tag=$$(git describe --tags --abbrev=0); \
	echo "Latest tag: $$latest_tag"; \
	ver=$$(echo $$latest_tag | sed 's/^v//'); \
	major=$$(echo $$ver | cut -d. -f1); \
	minor=$$(echo $$ver | cut -d. -f2); \
	patch=$$(echo $$ver | cut -d. -f3); \
	if [ "$(PART)" = "major" ]; then \
		new_major=$$(($$major + 1)); \
		new_tag="v$${new_major}.0.0"; \
		msg="major increment"; \
	elif [ "$(PART)" = "minor" ]; then \
		new_minor=$$(($$minor + 1)); \
		new_tag="v$${major}.$${new_minor}.0"; \
		msg="minor increment"; \
	elif [ "$(PART)" = "patch" ]; then \
		new_patch=$$(($$patch + 1)); \
		new_tag="v$${major}.$${minor}.$${new_patch}"; \
		msg="patch increment"; \
	else \
		echo "Unknown PART: $(PART)"; exit 1; \
	fi; \
	echo "New tag: $$new_tag"; \
	read -p "Tag with '$$new_tag'? [y/N] " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		git tag -a "$$new_tag" -m "Release $$new_tag ($$msg)"; \
		echo "Tagged $$new_tag"; \
	else \
		echo "Aborted."; \
	fi

tag-major:
	$(MAKE) _tag PART=major

tag-minor:
	$(MAKE) _tag PART=minor

tag-patch:
	$(MAKE) _tag PART=patch

.DEFAULT_GOAL := help 
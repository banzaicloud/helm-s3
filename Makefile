# A Self-Documenting Makefile:
# http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html

# Generic.
ORGANIZATION ?= $$(basename $$(dirname $${PWD}))
REPOSITORY ?= $$(basename $${PWD})

# ---

# Container.
CONTAINER_IMAGE_NAME = $(ORGANIZATION)/$(REPOSITORY)

# Git.
GIT_DEFAULT_BRANCH = origin/main
GIT_REF = $$(git show-ref --head | awk '/HEAD/ {print $$1}')

# Go.
# Note: explicitly setting GOBIN for global build install (required for GitHub
# Actions environment).
export GOBIN ?= $(shell go env GOPATH)/bin
GO_ROOT_MODULE_PKG ?= $$(awk 'NR == 1 {print $$2 ; exit}' go.mod)

# Helm S3 plugin.
HELM_S3_PLUGIN_LATEST_VERSION ?= $$(awk '/^version:/ {print $$2 ; exit}' plugin.yaml)
HELM_S3_PLUGIN_VERSION ?= $(GIT_REF)

.PHONY: all
all: analyze build ## all runs the entire toolchain configured for local development.

.PHONY: analyze
analyze: ## analyze runs the code analysis tools for new code.
	@ echo "- Analyzing new code"
	@ golangci-lint run --new-from-rev $(GIT_DEFAULT_BRANCH) ./...

.PHONY: analyze-full
analyze-full: ## analyze-full runs the code analysis tools for all code.
	@ echo "- Analyzing code"
	@ golangci-lint run ./...

.PHONY: build
build: ## build builds the local packages. You can set the version through the HELM_S3_PLUGIN_VERSION environment variable, defaults to 'local'.
	@ echo "- Building project binaries and libraries"
	@ go install -ldflags "-X main.version=$(HELM_S3_PLUGIN_VERSION)" ./...
	@ export GOBIN="$${PWD}/bin" ; go install -ldflags "-X main.version=$(HELM_S3_PLUGIN_VERSION)" ./...

.PHONY: build-container
build-container: ## build-container builds the project's container with the ${VERSION} tag (defaults to local).
	@ echo "- Building container"
	@ docker build --tag "$(CONTAINER_IMAGE_NAME):$(HELM_S3_PLUGIN_VERSION)" .

.PHONY: build-latest
build-latest: HELM_S3_PLUGIN_VERSION=$(HELM_S3_PLUGIN_LATEST_VERSION) ## build-latest builds the local packages with the latest version based on the plugin.yaml.
build-latest: build

.PHONY: help
help: ## help displays the help message.
	@ grep -E '^[0-9a-zA-Z_-]+:.*## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

.PHONY: run-container
run-container: ## run-container runs the projects container in a throw-away context with the ${CMD} command as argument.
	@ echo "- Running container"
	@ docker run --interactive --rm --tty "$(CONTAINER_IMAGE_NAME):$(HELM_S3_PLUGIN_VERSION)" $(CMD)

.PHONY: test-unit
test-unit: ## test-unit runs the unit tests in the repository.
	@ echo "- Running unit tests"
	@ go test -count 1 -race $$(go list ./... | grep -v $(GO_ROOT_MODULE_PKG)/test/e2e)

.PHONY: test-e2e
test-e2e:
	go test -v ./tests/e2e/...

.PHONY: test-e2e-local
test-e2e-local:
	@ ./hack/test-e2e-local.sh

.PHONY: vendor
vendor: ## vendor downloads the dependencies to a local vendor folder.
	@ go mod vendor

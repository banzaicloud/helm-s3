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
HELM_S3_PLUGIN_NAME ?= s3
HELM_S3_PLUGIN_VERSION ?= $(GIT_REF)

# LocalStack.
LOCALSTACK_HEALTH = $$( \
	( curl -s $(LOCALSTACK_URL)/health || echo "{}" ) \
	| jq --arg ls_services "$(LOCALSTACK_SERVICES)" \
		'{ "services": ( $$ls_services | split(",") | sort | reduce .[] as $$service ({}; . + { ($$service): "down" } ) ) } * .' \
)
LOCALSTACK_HEALTH_LINE_COUNT = $$(echo "$(LOCALSTACK_HEALTH)" | jq | wc -l | grep -E -o "[0-9]+")
export LOCALSTACK_SERVICES ?= s3
LOCALSTACK_STATUS = $$(docker-compose ps | awk '$$1 ~ /localstack_main/ {found=1 ; print $$3} ; END { if (!found) {print "Down" } }')
LOCALSTACK_URL ?= http://localhost.localstack.cloud:4566

.PHONY: all
all: analyze build test ## all runs the entire toolchain configured for local development.

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

.PHONY: check-clean-git-state
check-git-state-clean: ## check-git-state-clean ensures the Git state is clean (no file changes occurred compared to the current reference).
	@ echo "- Checking Git state cleanliness"
	@ if [ "$$(git status --porcelain)" != "" ]; then \
		printf >&2 "Git state is not clean, not proceeding.\n\n" "$$(git diff)" ; \
		exit 1 ; \
	fi

.PHONY: check-go-mod-integrity
check-go-mod-integrity: check-git-state-clean ## check-go-mod-integrity checks wether the source code and the recorded go mod dependencies are in sync.
	@ echo "- Checking Go module dependencies integrity"
	@ go mod tidy
	@ if [ "$$(git status --porcelain)" != "" ]; then \
		printf >&2 '\n`go mod tidy` results in a dirty state, Go mod files are not in sync with the source code files, differences:\n\n%s\n\n' "$$(git diff)" ; \
		git reset --hard ; \
		exit 1 ; \
	fi

.PHONY: get-alpine-versions-json
get-alpine-versions-json: ## get-alpine-versions-json retrieves the available Alpine Docker image versions as a JSON array.
	@ curl -sSL -H "Authorization: Bearer $$(curl -sSL "https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/alpine:pull" | jq --raw-output '.token')" https://registry.hub.docker.com/v2/library/alpine/tags/list | \
		jq '[ .tags[] | capture("^(?<major>[0-9]{1,7})\\.(?<minor>[0-9]+)\\.(?<patch>[0-9]+)$$"; "gins") ] | sort_by(.patch | tonumber) | sort_by(.minor | tonumber) | sort_by(.major | tonumber) | map(.major+"."+.minor+"."+.patch)'

.PHONY: get-go-versions-json
get-go-versions-json: ## get-go-versions-json retrieves the available Go versions as a JSON array.
	@ curl -sSL -H "Accept: application/vnd.github.v3+json" https://api.github.com/repos/golang/go/git/refs/tags/go | \
		jq '[ .[].ref | capture("^refs/tags/go(?<major>[0-9]+)(\\.(?<minor>[0-9]+))?(\\.(?<patch>[0-9]+))?$$"; "gins") | map_values(. // "0") ] | sort_by(.patch | tonumber) | sort_by(.minor | tonumber) | sort_by(.major | tonumber) | map(.major+"."+.minor+"."+.patch)'

.PHONY: get-golangci-lint-versions-json
get-golangci-lint-versions-json: ## get-golangci-lint-versions-json retrieves the available golangci-lint versions as a JSON array.
	@ curl -sSL -H "Accept: application/vnd.github.v3+json" https://api.github.com/repos/golangci/golangci-lint/git/refs/tags/v | \
		jq '[ .[].ref | capture("^refs/tags/v(?<major>[0-9]+)(\\.(?<minor>[0-9]+))?(\\.(?<patch>[0-9]+))?$$"; "gins") | map_values(. // "0") ] | sort_by(.patch | tonumber) | sort_by(.minor | tonumber) | sort_by(.major | tonumber) | map(.major+"."+.minor+"."+.patch)'


.PHONY: get-goreleaser-versions-json
get-goreleaser-versions-json: ## get-goreleaser-versions-json retrieves the available GoReleaser versions as a JSON array.
	@ curl -sSL -H "Accept: application/vnd.github.v3+json" https://api.github.com/repos/goreleaser/goreleaser/git/refs/tags/v | \
		jq '[ .[].ref | capture("^refs/tags/v(?<major>[0-9]+)(\\.(?<minor>[0-9]+))?(\\.(?<patch>[0-9]+))?$$"; "gins") | map_values(. // "0") ] | sort_by(.patch | tonumber) | sort_by(.minor | tonumber) | sort_by(.major | tonumber) | map(.major+"."+.minor+"."+.patch)'

.PHONY: get-helm-versions-json
get-helm-versions-json: ## get-helm-versions-json retrieves the available Helm versions as a JSON array.
	@ curl -sSL -H "Accept: application/vnd.github.v3+json" https://api.github.com/repos/helm/helm/git/refs/tags/v | \
		jq '[ .[].ref | capture("^refs/tags/v(?<major>[0-9]+)(\\.(?<minor>[0-9]+))?(\\.(?<patch>[0-9]+))?$$"; "gins") | map_values(. // "0") ] | sort_by(.patch | tonumber) | sort_by(.minor | tonumber) | sort_by(.major | tonumber) | map(.major+"."+.minor+"."+.patch)'

.PHONY: help
help: ## help displays the help message.
	@ grep -E '^[0-9a-zA-Z_-]+:.*## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

.PHONY: install-plugin-local
install-plugin-local: ## install-plugin-local installs the Helm plugin with 'local' version.
	@ echo "- Installing plugin locally"
	@ if helm plugin list | grep -q $(HELM_S3_PLUGIN_NAME); then \
		helm plugin remove $(HELM_S3_PLUGIN_NAME) ; \
	fi
	@ export HELM_PLUGIN_INSTALL_LOCAL=1 ; \
		helm plugin install .

.PHONY: reset-e2e-test-env
reset-e2e-test-env: teardown-e2e-test-env-force setup-e2e-test-env ## reset-e22e-test-env sets up a new end to end test environment.

.PHONY: run-container
run-container: ## run-container runs the projects container in a throw-away context with the ${CMD} command as argument.
	@ echo "- Running container"
	@ docker run --interactive --rm --tty "$(CONTAINER_IMAGE_NAME):$(HELM_S3_PLUGIN_VERSION)" $(CMD)

.PHONY: setup-e2e-test-env
setup-e2e-test-env: ## setup-e2e-test-env sets up the end to end test environment.
	@ echo "- Setting up end to end test environment"
	@ if [ "$(LOCALSTACK_STATUS)" != "Up" ]; then \
		docker-compose up --detach ; \
	fi
	@ echo "Waithing for services to start:"
	@ printf "%b%b\n" "$(LOCALSTACK_HEALTH)" "\033[$(LOCALSTACK_HEALTH_LINE_COUNT)F"
	@ while $$(echo "$(LOCALSTACK_HEALTH)" | jq '.services | any(. != "running")') ; do \
		printf "%b%b\n" "$(LOCALSTACK_HEALTH)" "\033[$(LOCALSTACK_HEALTH_LINE_COUNT)F" ; \
		sleep 1 ; \
	done
	@ echo "$(LOCALSTACK_HEALTH)" | jq

.PHONY: status-e2e-test-env
status-e2e-test-env: ## status-e2e-test-env returns the current state of the end to end test environment.
	@ echo $(LOCALSTACK_STATUS)

.PHONY: status-e2e-test-env-localstack
status-e2e-test-env-localstack: ## status-e2e-test-env-localstack returns the current state of the end to end test environment's LocalStack instance.
	@ echo $(LOCALSTACK_HEALTH) | jq

.PHONY: teardown-e2e-test-env
teardown-e2e-test-env: ## teardown-e2e-test-env tears down the end to end test environment.
	@ echo "- Tearing down end to end test environment"
	@ if [ "$(LOCALSTACK_STATUS)" != "Down" ]; then \
		docker-compose down ; \
	fi

.PHONY: teardown-e2e-test-env-force
teardown-e2e-test-env-force: ## teardown-e2e-test-env-force tears down the end to end test environment. (Required to run 2 teardowns in a single rule.)
	@ echo "- Tearing down end to end test environment"
	@ docker-compose down

.PHONY: test
test: test-unit test-e2e ## test runs all tests in the repository.

.PHONY: test-coverage
test-coverage: ## test-coverage generates data about the test coverage percentage of the code.
	@ go test -cover -covermode atomic $$(go list ./... | grep -v $(GO_ROOT_MODULE_PKG)/test/e2e) | \
		jq --raw-input --slurp \
			'[ split("\n") | .[] | select(. != "") | capture("(\\?|ok)\\s+(?<package>\\S+)\\s+((([0-9.hmnµs]+)|(\\(cached\\)))\\s+coverage: (?<coverage>[0-9]+.[0-9]+)% of statements|\\[no test files\\])"; "gins") | .coverage = ( .coverage // "0.0" | tonumber ) ] | reduce .[] as $$entry ({}; . + { ($$entry.package): ($$entry.coverage) }) | . as $$coverages | $$coverages * { "average": ( $$coverages | add * 1000 / length | round / 1000 ) }'

.PHONY: test-unit
test-unit: ## test-unit runs the unit tests in the repository.
	@ echo "- Running unit tests"
	@ go test -count 1 -race $$(go list ./... | grep -v $(GO_ROOT_MODULE_PKG)/test/e2e)

.PHONY: test-e2e
test-e2e: reset-e2e-test-env install-plugin-local test-e2e-no-env teardown-e2e-test-env ## test-e2e sets up the end to end testing environment and runs the end to end tests in the repository.

.PHONY: test-e2e-no-env
test-e2e-no-env: ## test-e2e-no-env runs the end to end tests without any modifications to the testing environment.
	@ echo "- Running end to end tests"
	@ go test -count 1 -v $(GO_ROOT_MODULE_PKG)/test/e2e

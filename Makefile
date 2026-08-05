GO_VERSION=$(shell grep "^go " go.mod | cut -f 2 -d ' ')

comma:=,
GO_BUILD_TAGS=netgo,sqlite_fts5$(if $(EXTRA_BUILD_TAGS),$(comma)$(EXTRA_BUILD_TAGS))

# Set global environment variables, required for most targets
export ND_ENABLEINSIGHTSCOLLECTOR=false

ifneq ("$(wildcard .git/HEAD)","")
GIT_SHA=$(shell git rev-parse --short HEAD)
GIT_TAG=$(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)-SNAPSHOT
else
GIT_SHA=source_archive
GIT_TAG=$(patsubst playmusic-%,v%,$(notdir $(PWD)))-SNAPSHOT
endif

SUPPORTED_PLATFORMS ?= linux/amd64,linux/arm64,linux/arm/v5,linux/arm/v6,linux/arm/v7,linux/386,linux/riscv64,darwin/amd64,darwin/arm64,windows/amd64,windows/386
IMAGE_PLATFORMS ?= $(shell echo $(SUPPORTED_PLATFORMS) | tr ',' '\n' | grep "linux" | grep -v "arm/v5" | tr '\n' ',' | sed 's/,$$//')
PLATFORMS ?= $(SUPPORTED_PLATFORMS)
DOCKER_TAG ?= hengi01/play-music:develop

GOLANGCI_LINT_VERSION ?= v2.12.0

setup: check_env download-deps install-golangci-lint setup-git ##@1_Run_First Install dependencies and prepare development environment
.PHONY: setup

dev: check_env   ##@Development Start Play Music in development mode, with hot-reload for backend
	npx foreman -j Procfile.dev -p 4533 start
.PHONY: dev

server: check_go_env ##@Development Start the backend in development mode
	go tool reflex -d none -c reflex.conf
.PHONY: server

stop: ##@Development Stop development servers
	@echo "Stopping development servers..."
	@-pkill -f "go tool reflex.*reflex.conf"
	@-pkill -f "go run.*netgo"
	@echo "Development servers stopped."
.PHONY: stop

watch: ##@Development Start Go tests in watch mode (re-run when code changes)
	go tool ginkgo watch -tags=$(GO_BUILD_TAGS) -notify ./...
.PHONY: watch

PKG ?= ./...
test: ##@Development Run Go tests. Use PKG variable to specify packages to test, e.g. make test PKG=./server
	go test -tags $(GO_BUILD_TAGS) $(PKG)
.PHONY: test

testall: test ##@Development Run Go tests
.PHONY: testall

test-race: ##@Development Run Go tests with race detector
	go test -tags $(GO_BUILD_TAGS) -race -shuffle=on  $(PKG)
.PHONY: test-race

install-golangci-lint: ##@Development Install golangci-lint if not present
	@INSTALL=false; \
	if PATH=./bin:$$PATH which golangci-lint > /dev/null 2>&1; then \
		CURRENT_VERSION=$$(PATH=./bin:$$PATH golangci-lint version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n1); \
		REQUIRED_VERSION=$$(echo "$(GOLANGCI_LINT_VERSION)" | sed 's/^v//'); \
		if [ "$$CURRENT_VERSION" != "$$REQUIRED_VERSION" ]; then \
			echo "Found golangci-lint $$CURRENT_VERSION, but $$REQUIRED_VERSION is required. Reinstalling..."; \
			rm -f ./bin/golangci-lint; \
			INSTALL=true; \
		fi; \
	else \
		INSTALL=true; \
	fi; \
	if [ "$$INSTALL" = "true" ]; then \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s $(GOLANGCI_LINT_VERSION); \
	fi
.PHONY: install-golangci-lint

lint: install-golangci-lint ##@Development Lint Go code
	PATH=./bin:$$PATH golangci-lint run --timeout 5m
.PHONY: lint

lintall: lint ##@Development Lint Go code
.PHONY: lintall

format: ##@Development Format code
	@go tool goimports -w `find . -name '*.go' | grep -v _gen.go$$ | grep -v .pb.go$$`
	@go mod tidy
.PHONY: format

wire: check_go_env ##@Development Update Dependency Injection
	go tool wire gen -tags="$$(echo '$(GO_BUILD_TAGS)' | tr ',' ' ')" ./...
.PHONY: wire

gen: check_go_env ##@Development Run go generate for code generation
	go generate ./...
.PHONY: gen

migration-sql: ##@Development Create an empty SQL migration file
	@if [ -z "${name}" ]; then echo "Usage: make migration-sql name=name_of_migration_file"; exit 1; fi
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir db/migrations create ${name} sql
.PHONY: migration

migration-go: ##@Development Create an empty Go migration file
	@if [ -z "${name}" ]; then echo "Usage: make migration-go name=name_of_migration_file"; exit 1; fi
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir db/migrations create ${name}
.PHONY: migration

setup-dev: setup
.PHONY: setup-dev

setup-git: ##@Development Setup Git hooks (pre-commit and pre-push)
	@echo Setting up git hooks
	@mkdir -p .git/hooks
	@(cd .git/hooks && ln -sf ../../git/* .)
.PHONY: setup-git

build: check_go_env ##@Build Build the project (UI assets are embedded in the binary)
	go build -ldflags="-X github.com/hensi01/play-music/consts.gitSha=$(GIT_SHA) -X github.com/hensi01/play-music/consts.gitTag=$(GIT_TAG)" -tags=$(GO_BUILD_TAGS)
.PHONY: build

buildall: deprecated build
.PHONY: buildall

debug-build: check_go_env ##@Build Build the project (with remote debug on)
	go build -gcflags="all=-N -l" -ldflags="-X github.com/hensi01/play-music/consts.gitSha=$(GIT_SHA) -X github.com/hensi01/play-music/consts.gitTag=$(GIT_TAG)" -tags=$(GO_BUILD_TAGS)
.PHONY: debug-build

docker-platforms: ##@Cross_Compilation List supported platforms
	@echo "Supported platforms:"
	@echo "$(SUPPORTED_PLATFORMS)" | tr ',' '\n' | sort | sed 's/^/    /'
	@echo "\nUsage: make PLATFORMS=\"linux/amd64\" docker-build"
	@echo "       make IMAGE_PLATFORMS=\"linux/amd64\" docker-image"
.PHONY: docker-platforms

docker-build: ##@Cross_Compilation Cross-compile for any supported platform (check `make docker-platforms`)
	docker buildx build \
		--platform $(PLATFORMS) \
		--build-arg GIT_TAG=${GIT_TAG} \
		--build-arg GIT_SHA=${GIT_SHA} \
		--output "./binaries" --target binary .
.PHONY: docker-build

docker-image: ##@Cross_Compilation Build Docker image, tagged as `hensi01/play-music:develop`, override with DOCKER_TAG var. Use IMAGE_PLATFORMS to specify target platforms
	@echo $(IMAGE_PLATFORMS) | grep -q "windows" && echo "ERROR: Windows is not supported for Docker builds" && exit 1 || true
	@echo $(IMAGE_PLATFORMS) | grep -q "darwin" && echo "ERROR: macOS is not supported for Docker builds" && exit 1 || true
	@echo $(IMAGE_PLATFORMS) | grep -q "arm/v5" && echo "ERROR: Linux ARMv5 is not supported for Docker builds" && exit 1 || true
	docker buildx build \
		--platform $(IMAGE_PLATFORMS) \
		--build-arg GIT_TAG=${GIT_TAG} \
		--build-arg GIT_SHA=${GIT_SHA} \
		--tag $(DOCKER_TAG) .
.PHONY: docker-image

docker-run: ##@Development Run a Play Music Docker image. Usage: make docker-run tag=<tag>
	@if [ -z "$(tag)" ]; then echo "Usage: make docker-run tag=<tag>"; exit 1; fi
	@TAG_DIR="tmp/$$(echo '$(tag)' | tr '/:' '_')"; mkdir -p "$$TAG_DIR"; \
    VOLUMES="-v $(PWD)/$$TAG_DIR:/data"; \
	if [ -f playmusic.toml ]; then \
		VOLUMES="$$VOLUMES -v $(PWD)/playmusic.toml:/data/playmusic.toml:ro"; \
		MUSIC_FOLDER=$$(grep '^MusicFolder' playmusic.toml | head -n1 | sed 's/.*= *"//' | sed 's/".*//'); \
		if [ -n "$$MUSIC_FOLDER" ] && [ -d "$$MUSIC_FOLDER" ]; then \
		  VOLUMES="$$VOLUMES -v $$MUSIC_FOLDER:/music:ro"; \
	  	fi; \
	fi; \
	echo "Running: docker run --rm -p 4533:4533 $$VOLUMES $(tag)"; docker run --rm -p 4533:4533 $$VOLUMES $(tag)
.PHONY: docker-run

package: docker-build ##@Cross_Compilation Create binaries and packages for ALL supported platforms
	@if [ -z `which goreleaser` ]; then echo "Please install goreleaser first: https://goreleaser.com/install/"; exit 1; fi
	goreleaser release -f release/goreleaser.yml --clean --skip=publish --snapshot
.PHONY: package

##########################################
#### Miscellaneous

clean:
	@rm -rf ./binaries ./dist
.PHONY: clean

release:
	@if [[ ! "${V}" =~ ^[0-9]+\.[0-9]+\.[0-9]+.*$$ ]]; then echo "Usage: make release V=X.X.X"; exit 1; fi
	go mod tidy
	@if [ -n "`git status -s`" ]; then echo "\n\nThere are pending changes. Please commit or stash first"; exit 1; fi
	make pre-push
	git tag v${V}
	git push origin v${V} --no-verify
.PHONY: release

download-deps:
	@echo Downloading Go dependencies...
	@go mod download
	@go mod tidy # To revert any changes made by the `go mod download` command
.PHONY: download-deps

check_env: check_go_env
.PHONY: check_env

check_go_env:
	@(hash go) || (echo "\nERROR: GO environment not setup properly!\n"; exit 1)
	@current_go_version=`go version | cut -d ' ' -f 3 | cut -c3-` && \
		echo "$(GO_VERSION) $$current_go_version" | \
		tr ' ' '\n' | sort -V | tail -1 | \
		grep -q "^$${current_go_version}$$" || \
		(echo "\nERROR: Please upgrade your GO version\nThis project requires at least the version $(GO_VERSION)"; exit 1)
.PHONY: check_go_env

pre-push: lintall testall
.PHONY: pre-push

deprecated:
	@echo "WARNING: This target is deprecated and will be removed in future releases. Use 'make build' instead."
.PHONY: deprecated

.DEFAULT_GOAL := help

HELP_FUN = \
	%help; while(<>){push@{$$help{$$2//'options'}},[$$1,$$3] \
	if/^([\w-_]+)\s*:.*\#\#(?:@(\w+))?\s(.*)$$/}; \
	print"$$_:\n", map"  $$_->[0]".(" "x(20-length($$_->[0])))."$$_->[1]\n",\
	@{$$help{$$_}},"\n" for sort keys %help; \

help: ##@Miscellaneous Show this help
	@echo "Usage: make [target] ...\n"
	@perl -e '$(HELP_FUN)' $(MAKEFILE_LIST)

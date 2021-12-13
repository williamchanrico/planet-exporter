.PHONY: test clean all

NAME          := planet-exporter
BIN_DIRECTORY := ./bin
REVISION      := $(shell git rev-parse --short HEAD 2>/dev/null)
VERSION       := v0.2.0-dev

ifndef REVISION
	override REVISION = none
endif

ifndef VERSION
	override VERSION = v0.1.0
endif

APP_NAME     := $(NAME)
APP_VERSION  := $(VERSION)
APP_REVISION := $(REVISION)
APP_LDFLAGS  := -X 'main.version=$(APP_VERSION)' -X 'main.revision=$(APP_REVISION)'

DOCKER_IMAGE_TAG      := williamchanrico/$(NAME)
DOCKER_IMAGE_VERSION  := $(VERSION)
DOCKER_CONTAINER_NAME := $(NAME)

.DEFAULT_GOAL := help

# HELP
# This will output the help for each task
# Thanks to https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
.PHONY: help
help: ## This help
	@echo "Commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# GOLANG TASKS
# Build the Go app
.PHONY: go-build
go-build: ## Build the app (linux, use go-cross-build for other platforms)
	GO111MODULE=auto go build -o $(BIN_DIRECTORY)/$(APP_NAME) -ldflags "$(APP_LDFLAGS)" cmd/planet-exporter/main.go

.PHONY: go-run
go-run:  ## Run the app
	make go-build
	$(BIN_DIRECTORY)/$(APP_NAME) -h

.PHONY: go-cross-build
go-cross-build: ## Build the app for multiple platforms
	@mkdir -p $(BIN_DIRECTORY) || true
	@# darwin
	@for arch in "amd64" "386"; do \
		CGO_ENABLED=0 GOOS=darwin GOARCH=$${arch} make go-build; \
		sleep 0.5; \
		cd $(BIN_DIRECTORY); \
		tar czf $(APP_NAME)_$(APP_VERSION)_darwin_$${arch}.tar.gz $(APP_NAME); \
		cd ..; \
	done;

.PHONY: go-build-linux
go-build-linux: ## Build the app for linux platforms exclude: "arm64" CGO errors
	@for arch in "amd64" "386"; do \
		CGO_ENABLED=0 GOOS=linux GOARCH=$${arch} make go-build; \
		sleep 0.5; \
		cd $(BIN_DIRECTORY); \
		tar czf $(APP_NAME)_$(APP_VERSION)_linux_$${arch}.tar.gz $(APP_NAME); \
		cd ..; \
	done;
	@rm -rf $(BIN_DIRECTORY)/$(APP_NAME)

# DOCKER TASKS
# Build the container
.PHONY: docker-build
docker-build: ## Build the container
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make go-build; \
	docker build -t $(DOCKER_IMAGE_TAG):$(DOCKER_IMAGE_VERSION) -t $(DOCKER_IMAGE_TAG):latest .

.PHONY: docker-run
docker-run: ## Run container
	@make docker-stop || true
	docker run -dit -p 8080:80 --name $(DOCKER_CONTAINER_NAME) $(DOCKER_IMAGE_TAG):$(DOCKER_IMAGE_VERSION)
	@docker exec -it $(DOCKER_CONTAINER_NAME) /bin/bash
	@make docker-stop || true

.PHONY: docker-push
docker-push: ## Push image
	docker push $(DOCKER_IMAGE_TAG)

.PHONY: docker-stop
docker-stop: ## Stop container
	@docker rm -f $(DOCKER_CONTAINER_NAME) 2>/dev/null || true

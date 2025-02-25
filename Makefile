# Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
.PHONY: build fmt verify release lint vendor mod-tidy mod-vendor mod-verify check-vendor

GO_CMD ?= go
GO_FMT ?= gofmt
GO_SRC := $(shell find . -type f -name '*.go' -not -path "./vendor/*")

BINARY_NAME ?= holodeck

VERSION := 0.0.1
GINKGO_VERSION ?= $(shell $(GO_CMD) list -m -f '{{.Version}}' github.com/onsi/ginkgo/v2)

IMAGE_REGISTRY ?= ghcr.io/arangogutierrez
IMAGE_TAG_NAME ?= $(VERSION)
IMAGE_NAME := holodeck
IMAGE_REPO := $(IMAGE_REGISTRY)/$(IMAGE_NAME)
IMAGE_TAG := $(IMAGE_REPO):$(IMAGE_TAG_NAME)

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

# Apply go fmt to the codebase
fmt:
	go list -f '{{.Dir}}' $(MODULE)/... \
		| xargs gofmt -s -l -w

goimports:
	go list -f {{.Dir}} $(MODULE)/... \
		| xargs goimports -local $(MODULE) -w

lint:
	golangci-lint run ./...

mod-tidy:
	@for mod in $$(find . -name go.mod); do \
	    echo "Tidying $$mod..."; ( \
	        cd $$(dirname $$mod) && go mod tidy \
            ) || exit 1; \
	done

mod-verify:
	@for mod in $$(find . -name go.mod); do \
	    echo "Verifying $$mod..."; ( \
	        cd $$(dirname $$mod) && go mod verify | sed 's/^/  /g' \
	    ) || exit 1; \
	done

mod-vendor: mod-tidy
	@for mod in $$(find . -name go.mod -not -path "./deployments/*"); do \
	    echo "Vendoring $$mod..."; ( \
	        cd $$(dirname $$mod) && go mod vendor \
	    ) || exit 1; \
	done

vendor: mod-vendor

check-modules: | mod-tidy mod-verify mod-vendor
	git diff --quiet HEAD -- $$(find . -name go.mod -o -name go.sum -o -name vendor)

build-action:
	@rm -rf bin
	GOOS=linux $(GO_CMD) build -o /go/bin/$(BINARY_NAME) cmd/action/main.go

build-cli:
	@rm -rf bin
	$(GO_CMD) build -o bin/$(BINARY_NAME) cmd/cli/main.go

verify:
	@out=`$(GO_FMT) -w -l -d $$(find . -name '*.go')`; \
	if [ -n "$$out" ]; then \
	    echo "$$out"; \
	    exit 1; \
	fi

release:
	@rm -rf bin
	@mkdir -p bin
	@for os in linux darwin; do \
		for arch in amd64 arm64; do \
			echo "Building $$os-$$arch"; \
			GOOS=$$os GOARCH=$$arch $(GO_CMD) build -o bin/$(BINARY_NAME)-action-$$os-$$arch cmd/action/main.go; \
		done; \
	done
	@for os in linux darwin; do \
		for arch in amd64 arm64; do \
			echo "Building $$os-$$arch"; \
			GOOS=$$os GOARCH=$$arch $(GO_CMD) build -o bin/$(BINARY_NAME)-$$os-$$arch cmd/cli/main.go; \
		done; \
	done

.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

CONTROLLER_GEN = $(PROJECT_DIR)/bin/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

GINKGO = $(PROJECT_DIR)/bin/ginkgo
.PHONY: ginkgo
ginkgo: ## Download ginkgo locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

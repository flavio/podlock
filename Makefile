CONTROLLER_TOOLS_VERSION := v0.16.5
ENVTEST_VERSION := release-0.19
ENVTEST_K8S_VERSION := 1.31.0

CONTROLLER_GEN ?= go run sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= go run sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)
HELM_SCHEMA ?= go run github.com/dadav/helm-schema/cmd/helm-schema@latest

GO_MOD_SRCS := go.mod go.sum
GO_BUILD_ENV := CGO_ENABLED=0 GOOS=linux GOARCH=amd64

ENVTEST_DIR ?= $(shell pwd)/.envtest

REGISTRY ?= ghcr.io
REPO ?= flavio/podlock
TAG ?= latest

.PHONY: all
all: controller nri

.PHONY: test
test: vet ## Run tests.
	$(GO_BUILD_ENV) CGO_ENABLED=1 KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(ENVTEST_DIR) -p path)" go test $$(go list ./... | grep -v /e2e) -race -test.v -coverprofile coverage/cover.out -covermode=atomic

.PHONY: helm-unittest
helm-unittest:
	helm unittest charts/podlock --file "tests/**/*_test.yaml"

.PHONY: test-e2e
test-e2e: controller-image nri-image
	$(GO_BUILD_ENV) go test ./test/e2e/ -v

.PHONY: fmt
fmt:
	$(GO_BUILD_ENV) go fmt ./...

.PHOHY: lint
lint: golangci-lint
	$(GO_BUILD_ENV) $(GOLANGCI_LINT) run --verbose

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GO_BUILD_ENV) $(GOLANGCI_LINT) run --fix

.PHOHY: vet
vet:
	$(GO_BUILD_ENV) go vet ./...

CONTROLLER_SRC_DIRS := cmd/controller api internal/controller internal/webhook pkg/constants
CONTROLLER_GO_SRCS := $(shell find $(CONTROLLER_SRC_DIRS) -type f -name '*.go')
CONTROLLER_SRCS := $(GO_MOD_SRCS) $(CONTROLLER_GO_SRCS)
.PHONY: controller
controller: $(CONTROLLER_SRCS) vet
	$(GO_BUILD_ENV) go build -o ./bin/controller ./cmd/controller

.PHONY: controller-image
controller-image:
	docker build -f ./Dockerfile.controller \
		-t "$(REGISTRY)/$(REPO)/controller:$(TAG)" .
	@echo "Built $(REGISTRY)/$(REPO)/controller:$(TAG)"

NRI_SRC_DIRS := cmd/nri api internal/cmdutil internal/nri pkg/constants
NRI_GO_SRCS := $(shell find $(NRI_SRC_DIRS) -type f -name '*.go')
NRI_SRCS := $(GO_MOD_SRCS) $(NRI_GO_SRCS)
.PHONY: nri
nri: $(NRI_SRCS) vet
	$(GO_BUILD_ENV) go build -o ./bin/nri ./cmd/nri

.PHONY: nri-image
nri-image:
	docker build -f ./Dockerfile.nri \
		-t "$(REGISTRY)/$(REPO)/nri:$(TAG)" .
	@echo "Built $(REGISTRY)/$(REPO)/nri:$(TAG)"

SEAL_SRC_DIRS := cmd/seal api internal/cmdutil internal/seal pkg/constants
SEAL_GO_SRCS := $(shell find $(SEAL_SRC_DIRS) -type f -name '*.go')
SEAL_SRCS := $(GO_MOD_SRCS) $(SEAL_GO_SRCS)
.PHONY: seal
seal: $(SEAL_SRCS) vet
	$(GO_BUILD_ENV) go build -o ./bin/seal ./cmd/seal

SWAP_OCI_HOOK_SRC_DIRS := cmd/swap-oci-hook internal/nri
SWAP_OCI_HOOK_GO_SRCS := $(shell find $(SWAP_OCI_HOOK_SRC_DIRS) -type f -name '*.go')
SWAP_OCI_HOOK_SRCS := $(GO_MOD_SRCS) $(SWAP_OCI_HOOK_GO_SRCS)
.PHONY: swap-oci-hook
swap-oci-hook: $(SWAP_OCI_HOOK_SRCS) vet
	$(GO_BUILD_ENV) go build -o ./bin/swap-oci-hook ./cmd/swap-oci-hook

.PHONY: generate
generate: generate-controller generate-chart

.PHONY: generate-controller
generate-controller: manifests  ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(GO_BUILD_ENV) $(CONTROLLER_GEN) object paths="./api/v1alpha1"

.PHONY: manifests
manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects. We use yq to modify the generated files to match our naming and labels conventions.
	$(GO_BUILD_ENV) $(CONTROLLER_GEN) rbac:roleName=controller-role crd webhook paths="./api/v1alpha1"  paths="./internal/controller" output:crd:artifacts:config=charts/podlock/templates/crd output:rbac:artifacts:config=charts/podlock/templates/controller
	sed -i 's/controller-role/{{ include "podlock.fullname" . }}-controller/' charts/podlock/templates/controller/role.yaml
	sed -i '/metadata:/a\  labels:\n    {{ include "podlock.labels" . | nindent 4 }}\n    app.kubernetes.io/component: controller' charts/podlock/templates/controller/role.yaml
	for f in ./charts/podlock/templates/crd/*.yaml; do \
		sed -i '/^[[:space:]]*annotations:/a\    helm.sh\/resource-policy: keep' "$$f"; \
	done

.PHONY: generate-chart
generate-chart: ## Generate Helm chart values schema.
	$(HELM_SCHEMA)

.PHONY: docs
docs:
	npx antora antora-playbook.yml

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)

## Tool Versions
GOLANGCI_LINT_VERSION ?= v2.5.0

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef

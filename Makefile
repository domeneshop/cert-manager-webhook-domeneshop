OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

BUILD_DATE = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT = $(shell git rev-parse --short HEAD)
GIT_BRANCH = $(shell git rev-parse --symbolic-full-name --verify --quiet --abbrev-ref HEAD)

IMAGE_NAME := "domeneshop/cert-manager-webhook-domeneshop"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/_out

KUBEBUILDER_VERSION=2.3.1

TEST_ZONE_NAME ?= example.com.

test: test/kubebuilder test/hemllint
	TEST_ZONE_NAME=$(TEST_ZONE_NAME) go test -v ./

test/kubebuilder:
	curl -fsSL https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VERSION)/kubebuilder_$(KUBEBUILDER_VERSION)_$(OS)_$(ARCH).tar.gz -o kubebuilder-tools.tar.gz
	mkdir -p "$(OUT)/kubebuilder"
	tar -xvf kubebuilder-tools.tar.gz
	mv kubebuilder_$(KUBEBUILDER_VERSION)_$(OS)_$(ARCH)/bin "$(OUT)/kubebuilder/"
	rm kubebuilder-tools.tar.gz
	rm -R kubebuilder_$(KUBEBUILDER_VERSION)_$(OS)_$(ARCH)

test/helmlint:
	helm lint deploy/domeneshop-webhook

clean-kubebuilder:
	rm -Rf "$(OUT)/kubebuilder"

clean-helm:
	rm -Rf "$(OUT)/helm"

clean: clean-kubebuilder clean-helm

docker:
	docker build \
		--build-arg "BUILD_DATE=$(BUILD_DATE)" \
		--build-arg "GIT_COMMIT=$(GIT_COMMIT)" \
		--build-arg "GIT_BRANCH=$(GIT_BRANCH)" \
		-t "$(IMAGE_NAME):$(IMAGE_TAG)" .

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml:
	helm template \
	    --name example-webhook \
        --set image.repository=$(IMAGE_NAME) \
        --set image.tag=$(IMAGE_TAG) \
        deploy/cert-manager-webhook-domeneshop > "$(OUT)/rendered-manifest.yaml"

.PHONY: chart
chart: test/helmlint clean-helm
	helm package --destination "$(OUT)/helm" deploy/domeneshop-webhook
	helm repo index "$(OUT)/helm" \
		--url "https://awigen.github.io/cert-manager-webhook-domeneshop" \
		--merge "charts/index.yaml"
	mv -f "$(OUT)/helm/index.yaml" charts/
	mv -f "$(OUT)/helm/"*.tgz charts/


ORG ?= crazytaxii
TARGET_DIR ?= ./dist
BUILDX ?= false
PLATFORM ?= linux/amd64,linux/arm64
OS ?= linux
ARCH ?= amd64
TAG ?= latest

.PHONY: build image

build:
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o $(TARGET_DIR)/restarter ./cmd/restarter

image:
ifeq ($(BUILDX), false)
	docker build \
		--force-rm \
		--no-cache \
		-t $(ORG)/kube-cron-restarter:$(TAG) .
else
	docker buildx build \
		--force-rm \
		--no-cache \
		--platform $(PLATFORM) \
		--push \
		-t $(ORG)/kube-cron-restarter:$(TAG) .
endif

clean:
	rm -rf $(TARGET_DIR)

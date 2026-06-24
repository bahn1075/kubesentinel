# KubeSentinel AI — build & deploy helpers
# 이미지 레지스트리/태그는 환경에 맞게 오버라이드: make docker-push REGISTRY=... TAG=...

REGISTRY ?= ghcr.io/your-org
IMAGE    ?= $(REGISTRY)/kubesentinel-ai
FE_IMAGE ?= $(REGISTRY)/kubesentinel-ai-front
TAG      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: build test vet fmt lint docker-build docker-push \
        frontend-build frontend-docker-build frontend-docker-push \
        helm-lint helm-template

## Go --------------------------------------------------------------
build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w internal/ cmd/

# 검증 한 방 (CI에서 사용)
lint: fmt vet
	@test -z "$$(gofmt -l internal/ cmd/)" || (echo "gofmt issues" && exit 1)
	go build ./... && go test ./...

## Docker (multi-arch) ---------------------------------------------
# 로컬 단일 아키텍처 빌드
docker-build:
	docker build -t $(IMAGE):$(TAG) .

# 멀티아치 빌드 후 레지스트리 push (buildx 필요)
docker-push:
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE):$(TAG) --push .

## Frontend (별도 빌드) --------------------------------------------
frontend-build:
	cd frontend && npm ci && npm run build

frontend-docker-build:
	docker build -t $(FE_IMAGE):$(TAG) frontend

frontend-docker-push:
	docker buildx build --platform $(PLATFORMS) -t $(FE_IMAGE):$(TAG) --push frontend

## Helm ------------------------------------------------------------
helm-lint:
	helm lint charts/kubesentinel-ai

helm-template:
	helm template kubesentinel charts/kubesentinel-ai

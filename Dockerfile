# syntax=docker/dockerfile:1
#
# Multi-arch (amd64 + arm64) build. (architecture.md R2)
# 빌드 예시:
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     -t <registry>/kubesentinel-ai:<tag> --push .

# --- build stage ---
# BUILDPLATFORM에서 교차 컴파일하여 빌드 속도를 확보한다.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /src

# 의존성 매니페스트 먼저 복사 (레이어 캐시). 현재 외부 의존성은 없다.
COPY go.mod ./
RUN go mod download

COPY . .

# 대상 플랫폼으로 정적 바이너리 빌드 (CGO 비활성 → distroless static 사용 가능)
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" \
    -o /out/kubesentinel-ai ./cmd/kubesentinel-ai

# --- runtime stage ---
# distroless static(nonroot): CA 인증서 포함(HTTPS LLM/webhook 호출용), 쉘 없음.
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /out/kubesentinel-ai /usr/local/bin/kubesentinel-ai

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/kubesentinel-ai"]

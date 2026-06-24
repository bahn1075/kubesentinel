# syntax=docker/dockerfile:1
#
# Frontend는 백엔드와 별도로 빌드/푸시한다.
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     -t <registry>/kubesentinel-ai-frontend:<tag> --push frontend/

# --- build stage ---
FROM node:22-alpine AS builder
WORKDIR /app

# 의존성 레이어 캐시 (lockfile 기반 재현 가능 설치)
COPY package.json package-lock.json* ./
RUN npm ci

COPY . .

# 빌드 타임 주입(선택). 기본은 mock 모드.
ARG VITE_USE_MOCK=true
ARG VITE_API_BASE_URL=/api
ENV VITE_USE_MOCK=${VITE_USE_MOCK} VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN npm run build

# --- runtime stage ---
FROM nginxinc/nginx-unprivileged:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 8080

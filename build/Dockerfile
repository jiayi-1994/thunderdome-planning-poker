# syntax=docker/dockerfile:1

# Build assets once on the runner's native architecture.
FROM --platform=$BUILDPLATFORM node:24.17-slim AS builder-node
WORKDIR /webapp/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci --no-progress
COPY ui/ ./
RUN npm run build

# Cross-compile Go rather than emulating an entire ARM build.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder-go
ARG TARGETOS
ARG TARGETARCH
ARG BUILD_VERSION=dev
RUN apk add --no-cache git ca-certificates && adduser -D -g '' appuser
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd/ ./cmd/
COPY docs/ ./docs/
COPY internal/ ./internal/
COPY thunderdome/ ./thunderdome/
COPY *.go ./
COPY ui/*.go ./ui/
COPY --from=builder-node /webapp/ui/dist ./ui/dist
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X main.version=$BUILD_VERSION" -o /go/bin/thunderdome .

FROM scratch
COPY --from=builder-go /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder-go /etc/passwd /etc/passwd
COPY --from=builder-go /go/bin/thunderdome /go/bin/thunderdome
USER appuser
EXPOSE 8080
ENTRYPOINT ["/go/bin/thunderdome"]
CMD ["serve"]

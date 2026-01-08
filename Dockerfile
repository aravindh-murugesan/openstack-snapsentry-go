# Build the binary
FROM --platform=$BUILDPLATFORM golang:1.25 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

# Copy the full Go source
COPY . .

# Build
# the GOARCH has no default value to allow the binary to be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN MODULE=$(go list -m) && \
    PKG="${MODULE}/internal/cli" && \
    VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev") && \
    COMMIT=$(git rev-parse --short HEAD) && \
    DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags=" \
        -X '${PKG}.SnapsentryVersion=${VERSION}' \
        -X '${PKG}.SnapsentryCommit=${COMMIT}' \
        -X '${PKG}.SnapsentryDate=${DATE}' " \ 
        -a -o snapsentry-go cmd/main.go

# Use distroless as minimal base image to package the binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/snapsentry-go /snapsentry-go
USER 65532:65532

ENTRYPOINT ["/snapsentry-go"]

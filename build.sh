#!/bin/bash

VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
TAGS=$(git tag --points-at HEAD | xargs | tr ' ' ',')

PKG="github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cli"
BUILD_DIR="build/${VERSION}"
mkdir -p $BUILD_DIR

LDFLAGS="-X '${PKG}.SnapsentryVersion=${VERSION}' -X '${PKG}.SnapsentryCommit=${COMMIT}' -X '${PKG}.SnapsentryDate=${DATE}' -X '${PKG}.SnapsentryTags=${TAGS}' -s -w"

echo "Building for linux/amd64..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="${LDFLAGS}" -o ${BUILD_DIR}/snapsentry-linux-amd64 ./cmd/main.go

echo "Building for linux/arm64..."
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="${LDFLAGS}" -o ${BUILD_DIR}/snapsentry-linux-arm64 ./cmd/main.go

echo "Building for darwin/arm64..."
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="${LDFLAGS}" -o ${BUILD_DIR}/snapsentry-darwin-arm64 ./cmd/main.go

#!/bin/bash
set -e

APP_NAME="viri"
VERSION="0.1.0"
BUILD_DIR="build"
LDFLAGS="-ldflags \"-X main.Version=${VERSION} -s -w\""

echo "Building Viri v${VERSION} for all platforms..."
mkdir -p "${BUILD_DIR}"

build() {
    local goos=$1
    local goarch=$2
    local goarm=$3
    local ext=""
    local arm_suffix=""

    if [ "$goos" = "windows" ]; then
        ext=".exe"
    fi

    if [ -n "$goarm" ]; then
        arm_suffix="-armv${goarm}"
    fi

    echo "  ${goos}/${goarch}${arm_suffix}..."

    if [ -n "$goarm" ]; then
        GOOS=$goos GOARCH=$goarch GOARM=$goarm go build $LDFLAGS \
            -o "${BUILD_DIR}/${APP_NAME}d-${goos}-${goarch}${arm_suffix}${ext}" ./cmd/virid
        GOOS=$goos GOARCH=$goarch GOARM=$goarm go build $LDFLAGS \
            -o "${BUILD_DIR}/${APP_NAME}ctl-${goos}-${goarch}${arm_suffix}${ext}" ./cmd/virictl
    else
        GOOS=$goos GOARCH=$goarch go build $LDFLAGS \
            -o "${BUILD_DIR}/${APP_NAME}d-${goos}-${goarch}${ext}" ./cmd/virid
        GOOS=$goos GOARCH=$goarch go build $LDFLAGS \
            -o "${BUILD_DIR}/${APP_NAME}ctl-${goos}-${goarch}${ext}" ./cmd/virictl
    fi
}

echo ""
echo "Linux:"
build linux amd64 ""
build linux arm64 ""

echo ""
echo "macOS:"
build darwin amd64 ""
build darwin arm64 ""

echo ""
echo "Windows:"
build windows amd64 ""

echo ""
echo "Raspberry Pi:"
build linux arm 6
build linux arm 7

echo ""
echo "All builds complete!"
echo ""
ls -lh "${BUILD_DIR}/"

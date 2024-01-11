#!/bin/bash
set -xve

REPO_DIR="."
RELEASE_DIR="./dist"
DATE=`date +"%Y%m%d"`

# Create sub folder
mkdir -p ${RELEASE_DIR}/${DATE} || true

# Build x86 linux binary
CGO_ENABLED=0 GOARCH="amd64" GOOS="linux" go build -o ${RELEASE_DIR}/${DATE}/toriix_linux_amd64_${DATE} ${REPO_DIR}/*.go
# Build x86 windows binary
CGO_ENABLED=0 GOARCH="amd64" GOOS="windows" go build -o ${RELEASE_DIR}/${DATE}/toriix_windows_amd64_${DATE} ${REPO_DIR}/*.go
# Build arm linux binary
CGO_ENABLED=0 GOARCH="arm" GOOS="linux" go build -o ${RELEASE_DIR}/${DATE}/toriix_linux_arm_${DATE} ${REPO_DIR}/*.go

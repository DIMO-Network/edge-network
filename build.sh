#!/usr/bin/env bash
set -e

GOARCH=arm GOOS=linux go build -ldflags="-s -w" -o edge-network && upx edge-network 
#!/usr/bin/env bash

set -euo pipefail

echo "==> Running SNAP B2B Token Service Validation Scenarios..."

# 1. Check Go Test Suite
echo "1. Running TDD Go Test Suite with Race Detector..."
export GOPATH=$HOME/go
go test -race -v ./...

echo "2. Building Multi-Stage Non-Root Docker Image..."
docker build -t backbone-payment-gateway:latest .

echo "==> All SNAP Validation Checks Passed Successfully! 🎉"

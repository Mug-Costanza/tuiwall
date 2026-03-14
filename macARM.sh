#!/bin/bash
# Builds for Apple Silicon Macs
# Ensure termios_darwin.go is moved to cmd/tuiwall/
GOOS=darwin GOARCH=arm64 go build -o tuiwall-MacOS-arm64 ./cmd/tuiwall

#!/bin/bash
# Builds for Intel Macs
# Ensure termios_darwin.go is moved to cmd/tuiwall/
GOOS=darwin GOARCH=amd64 go build -o tuiwall-Darwin-amd64 ./cmd/tuiwall

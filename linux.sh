#!/bin/bash
# Builds for 64-bit Linux
# Ensure termios_linux.go is moved to cmd/tuiwall/
GOOS=linux GOARCH=amd64 go build -o tuiwall-linux-amd64 ./cmd/tuiwall

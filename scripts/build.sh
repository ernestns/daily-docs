#!/usr/bin/env sh
set -eu

mkdir -p bin
go build -o bin/dailydocs ./cmd/web

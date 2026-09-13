#!/usr/bin/env bash
set -euo pipefail

build_lambda() {
  local name=$1
  local dir="lambdas/${name}"

  echo "building ${name} lambda..."
  (cd "$dir" && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bootstrap main.go)
  (cd "$dir" && zip -j function.zip bootstrap && rm bootstrap)
  echo "built ${dir}/function.zip"
}

build_lambda "self-healer"
build_lambda "guardrail"
build_lambda "reaper"

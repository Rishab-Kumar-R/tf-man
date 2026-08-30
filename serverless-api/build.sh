#!/bin/bash
set -e
cd src
GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap main.go
zip -j ../lambda.zip bootstrap
rm bootstrap
cd ..
echo "built lambda.zip"

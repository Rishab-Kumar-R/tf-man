#!/bin/bash
set -e
if [ -z "$1" ]; then
    echo "usage: ./new-env.sh <workspace-name>"
    exit 1
fi

terraform workspace new "$1"
terraform apply -auto-approve

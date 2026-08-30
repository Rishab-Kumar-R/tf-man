#!/bin/bash
set -e
if [ -z "$1" ]; then
    echo "usage: ./switch-env.sh <workspace-name>"
    terraform workspace list
    exit 1
fi

terraform workspace select "$1"
terraform plan

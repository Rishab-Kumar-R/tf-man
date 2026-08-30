#!/bin/bash
set -e
if [ -z "$1" ]; then
    echo "usage: ./destroy-env.sh <workspace-name>"
    exit 1
fi

terraform workspace select "$1"
terraform destroy -auto-approve
terraform workspace select default
terraform workspace delete "$1"

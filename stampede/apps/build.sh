#!/usr/bin/env bash
set -euo pipefail

SERVICES=(edge-gateway catalog-service inventory-service order-service payment-worker fraud-detector notification-worker)

REGION="${AWS_REGION:-ap-south-1}"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGISTRY="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"
TAG=$(git rev-parse --short HEAD)

echo "logging in to ECR..."
aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "$REGISTRY"

for service in "${SERVICES[@]}"; do
    REPO="${REGISTRY}/stampede/${service}"
    echo "=== Building ${service} (${REPO}:${TAG}) ==="

    docker build \
        --platform linux/amd64 \
        --build-arg SERVICE="${service}" \
        -f Dockerfile \
        -t "${REPO}:${TAG}" \
        .

    docker push "${REPO}:${TAG}"
done

echo
echo "build succeeded. images pushed with tag ${TAG}:"
for service in "${SERVICES[@]}"; do
    echo "  ${REGISTRY}/stampede/${service}:${TAG}"
done
echo
echo "gitops/apps/*/deployment.yaml intentionally keep a placeholder image (no AWS"
echo "account ID committed to the public repo). Point the cluster at these real"
echo "images by running, from gitops/:"
echo "  ./apply-image-overrides.sh ${TAG}"

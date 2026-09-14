#!/usr/bin/env bash
set -euo pipefail

SERVICES=(edge-gateway catalog-service inventory-service order-service payment-worker fraud-detector notification-worker)

REGION="${AWS_REGION:-ap-south-1}"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGISTRY="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"
TAG=$(git rev-parse --short HEAD)

BUCKET=$(terraform -chdir=.. output -raw codebuild_source_bucket)
PROJECT=$(terraform -chdir=.. output -raw codebuild_project_name)

echo "zipping apps/ source..."
ZIP_PATH=$(mktemp -t stampede-source).zip
rm -f "$ZIP_PATH"
zip -rq "$ZIP_PATH" . -x ".git/*"

echo "uploading source to s3://${BUCKET}/source.zip..."
aws s3 cp "$ZIP_PATH" "s3://${BUCKET}/source.zip" --region "$REGION" >/dev/null
rm -f "$ZIP_PATH"

echo "starting CodeBuild (tag=${TAG})..."
BUILD_ID=$(aws codebuild start-build \
    --project-name "$PROJECT" \
    --environment-variables-override name=IMAGE_TAG,value="$TAG",type=PLAINTEXT \
    --region "$REGION" \
    --query 'build.id' --output text)

echo "build started: ${BUILD_ID}"
echo "waiting for it to finish (builds all 7 services in one run, usually a few minutes)..."

BUILD_STATUS="IN_PROGRESS"
while [ "$BUILD_STATUS" = "IN_PROGRESS" ]; do
    sleep 15
    BUILD_STATUS=$(aws codebuild batch-get-builds --ids "$BUILD_ID" --region "$REGION" --query 'builds[0].buildStatus' --output text)
    echo "status: ${BUILD_STATUS}"
done

if [ "$BUILD_STATUS" != "SUCCEEDED" ]; then
    echo
    echo "build did not succeed (status: ${BUILD_STATUS})."
    echo "logs: https://${REGION}.console.aws.amazon.com/codesuite/codebuild/${ACCOUNT_ID}/projects/${PROJECT}/build/${BUILD_ID//://}"
    exit 1
fi

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

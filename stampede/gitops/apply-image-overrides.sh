#!/usr/bin/env bash
set -euo pipefail

SERVICES=(edge-gateway catalog-service inventory-service order-service payment-worker fraud-detector notification-worker)

REGION="${AWS_REGION:-ap-south-1}"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGISTRY="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"
TAG="${1:-$(git rev-parse --short HEAD)}"
WAF_ARN=$(terraform -chdir=.. output -raw waf_web_acl_arn)

PATCH_FILE=$(mktemp)
trap 'rm -f "$PATCH_FILE"' EXIT

for service in "${SERVICES[@]}"; do
    cat > "$PATCH_FILE" <<EOF
spec:
  source:
    kustomize:
      images:
        - registry.invalid/stampede/${service}=${REGISTRY}/stampede/${service}:${TAG}
EOF
    kubectl patch application "$service" -n argocd --type merge --patch-file="$PATCH_FILE"
    echo "patched ${service} -> ${REGISTRY}/stampede/${service}:${TAG}"
done

echo "patching edge-gateway's WAF ACL ARN..."
cat > "$PATCH_FILE" <<EOF
spec:
  source:
    kustomize:
      images:
        - registry.invalid/stampede/edge-gateway=${REGISTRY}/stampede/edge-gateway:${TAG}
      patches:
        - target:
            kind: Ingress
            name: edge-gateway
          patch: |-
            - op: replace
              path: /metadata/annotations/alb.ingress.kubernetes.io~1wafv2-acl-arn
              value: ${WAF_ARN}
EOF
kubectl patch application edge-gateway -n argocd --type merge --patch-file="$PATCH_FILE"

echo
echo "done. ArgoCD will pick these up on its next sync (usually within a few minutes),"
echo "or force it now with: for s in ${SERVICES[*]}; do kubectl -n argocd patch application \$s --type merge -p '{\"operation\":{\"sync\":{}}}'; done"

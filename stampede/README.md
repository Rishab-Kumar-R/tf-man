# stampede

A flash-sale/auction-style microservices platform on EKS, built entirely from custom Terraform modules (no
`terraform-aws-modules/*` registry sources). Built as a capstone to demonstrate real production patterns end-to-end:
Karpenter autoscaling, GitOps via ArgoCD, IRSA least-privilege per service, a transactional outbox pattern, and real
distributed-systems correctness (idempotency, dedup, retries, timeouts, bulkheading) - not just "deploy some pods and
call it done."

## What it does

Seven Go microservices simulate a flash-sale checkout flow: a product catalog with full-text search, inventory
reservation with race-safe stock decrements, order creation, async fraud checks and payment processing fanned out over
SNS/SQS, and a transactional outbox so "did we save the order" and "did we tell the rest of the system" can never
diverge.

## AWS services used

| Service                          | Role                                                                                                                                |
|----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------|
| **EKS**                          | Kubernetes control plane running everything below                                                                                   |
| **EC2**                          | Managed node group + Karpenter-launched nodes, launch templates, security groups                                                    |
| **Elastic Load Balancing (ALB)** | Public entry point - provisioned dynamically by the AWS Load Balancer Controller from an Ingress, not a raw Terraform resource      |
| **WAF (WAFv2)**                  | Attached to the ALB - managed rule sets + rate limiting                                                                             |
| **RDS (Postgres)**               | Two isolated instances (`orders`, `catalog`) - split specifically so both services' identically-named `outbox` tables can't collide |
| **ElastiCache (Redis)**          | Inventory stock counters, dedup keys, WebSocket pub/sub for live stock updates                                                      |
| **OpenSearch**                   | Product catalog full-text search                                                                                                    |
| **SNS + SQS**                    | Event fan-out - order events published to SNS, consumed by per-service SQS subscriptions with filter policies                       |
| **ECR**                          | Immutable-tagged container images, git-SHA tagged                                                                                   |
| **CodeBuild**                    | Builds and pushes images remotely - no local Docker required                                                                        |
| **S3**                           | CodeBuild source bucket + Terraform state backend                                                                                   |
| **Secrets Manager**              | DB credentials, Grafana admin password                                                                                              |
| **CloudWatch**                   | Container Insights, application/host/dataplane log groups                                                                           |
| **IAM**                          | Per-service IRSA roles (least-privilege via OIDC), cluster/node roles                                                               |
| **VPC**                          | Subnets, route tables, NAT gateway, internet gateway                                                                                |

Karpenter, ArgoCD, and the AWS Load Balancer Controller are open-source Kubernetes controllers running in-cluster - not
AWS-native services, but core to how this platform actually operates.

## Repo layout

```
stampede/
├── modules/          # custom Terraform modules (one per concern - eks-cluster, rds, redis, opensearch, messaging, codebuild, ...)
├── apps/             # 7 Go microservices + a shared internal/ module (go.work workspace)
├── gitops/           # ArgoCD "app of apps" - platform/ (root app, Karpenter NodePool, per-service Applications) + apps/ (each service's Kustomize base)
├── environments.tf   # dev/staging/prod sizing, selected via terraform.workspace
└── main.tf           # root composition wiring every module together
```

## Architecture

- **GitOps**: `gitops/platform/root-app.yaml` is the root ArgoCD Application, scoped to `platform/`. It spawns 7
  per-service leaf Applications (`gitops/platform/apps/*.yaml`), each independently syncing its own Kustomize base under
  `gitops/apps/<service>/`.
- **Services** (`apps/`): `edge-gateway` (public API gateway + WebSocket), `order-service`, `catalog-service`,
  `inventory-service`, `payment-worker`, `fraud-detector`, `notification-worker`.
- **Autoscaling**: managed node group provides baseline capacity; Karpenter provisions burst capacity on demand. Its
  `NodePool`/`EC2NodeClass` is applied automatically by a `null_resource` right after `terraform apply` - no separate
  manual `kubectl apply` step.

## Deploying

```bash
# 1. Terraform infra (from stampede/)
cp backend.hcl.example backend.hcl   # fill in your own state bucket name
terraform init -backend-config=backend.hcl
terraform apply

# 2. Build and push service images (no local Docker needed - builds on CodeBuild)
cd apps
./build-remote.sh

# 3. Bootstrap ArgoCD
kubectl apply -f ../gitops/platform/root-app.yaml

# 4. Point the live cluster at the real ECR images (never committed to git - see below)
cd ../gitops
./apply-image-overrides.sh
```

### Why images aren't in git

`gitops/apps/*/deployment.yaml` intentionally reference a placeholder image (`registry.invalid/...`), not your real ECR
registry - this repo is public, and an ECR URI bakes in your AWS account ID. `apply-image-overrides.sh` patches the real
registry + tag directly onto the live ArgoCD `Application` objects in your cluster (never into a committed file). Re-run
it after every `build-remote.sh`.

## Accessing the API

Public entry point is `edge-gateway`, exposed via the ALB Ingress (WAF-protected):

```bash
kubectl get ingress edge-gateway -n stampede -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'
```

Every other service (`order-service`, `catalog-service`, `inventory-service`, `payment-worker`, `fraud-detector`,
`notification-worker`) is internal-only - reach them via:

```bash
kubectl port-forward -n stampede svc/<service> <local-port>:8080
```

## Logs

```bash
# live tail
kubectl logs -n stampede -l app=edge-gateway --tail=50 -f

# CloudWatch (persisted, searchable)
aws logs tail /aws/containerinsights/stampede-cluster/application --follow --region ap-south-1
```

## Monitoring

Grafana (via `kube-prometheus-stack`):

```bash
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80
# admin password:
aws secretsmanager get-secret-value --secret-id stampede-grafana-admin --region ap-south-1 --query SecretString --output text
```

## Instance sizing

`dev` runs `c7i-flex.large` (2 vCPU / 4GB) for both the managed node group and Karpenter. `t3.small` was tried first but
proved undersized once real workload + platform components ran concurrently - CPU-credit exhaustion and OOM under load
caused repeated node failures. `staging`/`prod` step up to `c7i-flex.xlarge`/`c7i-flex.2xlarge` respectively - see
`environments.tf` for full per-environment sizing.

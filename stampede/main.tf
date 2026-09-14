module "tags" {
  source = "./modules/tags"

  project     = var.project_name
  environment = terraform.workspace
  owner       = "rishabkumar"
}

locals {
  tags = module.tags.tags

  cluster_name = "${var.project_name}-cluster"
}

module "network" {
  source = "./modules/network"

  project_name       = var.project_name
  cluster_name       = local.cluster_name
  azs                = var.azs
  single_nat_gateway = local.env.single_nat_gateway
  tags               = local.tags
}

module "eks_cluster" {
  source = "./modules/eks-cluster"

  cluster_name        = local.cluster_name
  vpc_id              = module.network.vpc_id
  private_subnet_ids  = module.network.private_subnet_ids
  public_subnet_ids   = module.network.public_subnet_ids
  node_instance_types = local.env.node_instance_types
  node_desired_size   = local.env.node_desired_size
  node_min_size       = local.env.node_min_size
  node_max_size       = local.env.node_max_size
  tags                = local.tags
}

module "messaging" {
  source       = "./modules/messaging"
  project_name = var.project_name

  subscribers = {
    fraud-detector = {}
    payment-worker = {
      filter_policy = jsonencode({ event_type = ["OrderPlaced"] })
    }
    notification-worker = {
      filter_policy = jsonencode({ event_type = ["OrderConfirmed", "FraudFlagged", "PaymentFailed"] })
    }
    order-service = {
      filter_policy = jsonencode({ event_type = ["OrderConfirmed", "PaymentFailed"] })
    }
  }

  tags = module.tags.tags
}

data "aws_caller_identity" "current" {}

module "alb_controller_irsa" {
  source = "./modules/irsa"

  role_name            = "${var.project_name}-alb-controller-role"
  oidc_provider_arn    = module.eks_cluster.oidc_provider_arn
  oidc_provider_url    = module.eks_cluster.oidc_provider_url
  namespace            = "kube-system"
  service_account_name = "aws-load-balancer-controller"
  policy_json          = file("${path.root}/policies/alb-controller-policy.json")
  tags                 = local.tags
}

module "ingress" {
  source = "./modules/ingress"

  cluster_name   = local.cluster_name
  irsa_role_arn  = module.alb_controller_irsa.role_arn
  vpc_id         = module.network.vpc_id
  waf_rate_limit = local.env.waf_rate_limit
  tags           = local.tags
}

module "container_insights_irsa" {
  source = "./modules/irsa"

  role_name            = "${var.project_name}-container-insights-role"
  oidc_provider_arn    = module.eks_cluster.oidc_provider_arn
  oidc_provider_url    = module.eks_cluster.oidc_provider_url
  namespace            = "amazon-cloudwatch"
  service_account_name = "cloudwatch-agent"
  managed_policy_arns  = ["arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"]
  tags                 = local.tags
}

module "autoscaling" {
  source = "./modules/autoscaling"

  project_name              = var.project_name
  cluster_name              = local.cluster_name
  cluster_endpoint          = module.eks_cluster.cluster_endpoint
  oidc_provider_arn         = module.eks_cluster.oidc_provider_arn
  oidc_provider_url         = module.eks_cluster.oidc_provider_url
  vpc_id                    = module.network.vpc_id
  private_subnet_ids        = module.network.private_subnet_ids
  cluster_security_group_id = module.eks_cluster.cluster_security_group_id
  nodepool_manifest_path    = "${path.root}/gitops/platform/karpenter-nodepool.yaml"
  tags                      = local.tags

  depends_on = [module.eks_cluster]
}

module "observability" {
  source = "./modules/observability"

  project_name                = var.project_name
  cluster_name                = local.cluster_name
  container_insights_role_arn = module.container_insights_irsa.role_arn
  tags                        = local.tags

  depends_on = [module.ingress, module.autoscaling]
}

module "gitops" {
  source = "./modules/gitops"

  tags = local.tags

  depends_on = [module.ingress, module.autoscaling]
}

module "rds_orders" {
  source = "./modules/rds"

  project_name               = "${var.project_name}-orders"
  vpc_id                     = module.network.vpc_id
  private_subnet_ids         = module.network.private_subnet_ids
  allowed_security_group_ids = [module.eks_cluster.cluster_security_group_id]
  instance_class             = local.env.rds_instance_class
  multi_az                   = local.env.rds_multi_az
  tags                       = local.tags
}

module "rds_catalog" {
  source = "./modules/rds"

  project_name               = "${var.project_name}-catalog"
  vpc_id                     = module.network.vpc_id
  private_subnet_ids         = module.network.private_subnet_ids
  allowed_security_group_ids = [module.eks_cluster.cluster_security_group_id]
  instance_class             = local.env.rds_instance_class
  multi_az                   = local.env.rds_multi_az
  tags                       = local.tags
}

module "redis" {
  source = "./modules/redis"

  project_name               = var.project_name
  vpc_id                     = module.network.vpc_id
  private_subnet_ids         = module.network.private_subnet_ids
  allowed_security_group_ids = [module.eks_cluster.cluster_security_group_id]
  node_type                  = local.env.redis_node_type
  num_cache_clusters         = local.env.redis_num_cache_clusters
  automatic_failover_enabled = local.env.redis_automatic_failover_enabled
  tags                       = local.tags
}

resource "kubernetes_namespace_v1" "stampede" {
  metadata {
    name = "stampede"
  }
}

resource "kubernetes_config_map_v1" "app_config" {
  metadata {
    name      = "stampede-config"
    namespace = kubernetes_namespace_v1.stampede.metadata[0].name
  }

  data = {
    EVENTS_TOPIC_ARN      = module.messaging.topic_arn
    REDIS_ADDR            = "${module.redis.primary_endpoint_address}:${module.redis.port}"
    CONCURRENCY           = "10"
    ORDER_DB_SECRET_ARN   = module.rds_orders.secret_arn
    CATALOG_DB_SECRET_ARN = module.rds_catalog.secret_arn

    FRAUD_DETECTOR_QUEUE_URL       = module.messaging.queue_urls["fraud-detector"]
    PAYMENT_WORKER_QUEUE_URL       = module.messaging.queue_urls["payment-worker"]
    NOTIFICATION_WORKER_QUEUE_URL  = module.messaging.queue_urls["notification-worker"]
    ORDER_SERVICE_STATUS_QUEUE_URL = module.messaging.queue_urls["order-service"]

    INVENTORY_SERVICE_URL = "http://inventory-service.stampede.svc.cluster.local:8080"
    ORDER_SERVICE_URL     = "http://order-service.stampede.svc.cluster.local:8080"
    CATALOG_SERVICE_URL   = "http://catalog-service.stampede.svc.cluster.local:8080"

    OPENSEARCH_ENDPOINT = "https://${module.opensearch.domain_endpoint}"

    FRAUD_THRESHOLD      = "20"
    FRAUD_WINDOW_SECONDS = "60"

    SIMULATED_FAILURE_RATE = "0.0"
  }
}

module "fraud_detector_irsa" {
  source = "./modules/irsa"

  role_name            = "${var.project_name}-fraud-detector-role"
  oidc_provider_arn    = module.eks_cluster.oidc_provider_arn
  oidc_provider_url    = module.eks_cluster.oidc_provider_url
  namespace            = "stampede"
  service_account_name = "fraud-detector"

  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "AllowOwnQueueConsume"
        Effect   = "Allow"
        Action   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
        Resource = module.messaging.queue_arns["fraud-detector"]
      },
      {
        Sid      = "AllowPublishFraudFlagged"
        Effect   = "Allow"
        Action   = "sns:Publish"
        Resource = module.messaging.topic_arn
      }
    ]
  })

  tags = local.tags
}

resource "kubernetes_service_account_v1" "fraud_detector" {
  metadata {
    name      = "fraud-detector"
    namespace = kubernetes_namespace_v1.stampede.metadata[0].name

    annotations = {
      "eks.amazonaws.com/role-arn" = module.fraud_detector_irsa.role_arn
    }
  }
}

module "opensearch" {
  source = "./modules/opensearch"

  project_name               = var.project_name
  vpc_id                     = module.network.vpc_id
  private_subnet_ids         = module.network.private_subnet_ids
  allowed_security_group_ids = [module.eks_cluster.cluster_security_group_id]
  allowed_principal_arns     = [module.catalog_service_irsa.role_arn]
  instance_type              = local.env.opensearch_instance_type
  instance_count             = local.env.opensearch_instance_count
  zone_awareness_enabled     = local.env.opensearch_zone_awareness_enabled
  tags                       = local.tags
}

module "payment_worker_irsa" {
  source = "./modules/irsa"

  role_name            = "${var.project_name}-payment-worker-role"
  oidc_provider_arn    = module.eks_cluster.oidc_provider_arn
  oidc_provider_url    = module.eks_cluster.oidc_provider_url
  namespace            = "stampede"
  service_account_name = "payment-worker"

  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "AllowOwnQueueConsume"
        Effect   = "Allow"
        Action   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
        Resource = module.messaging.queue_arns["payment-worker"]
      },
      {
        Sid      = "AllowPublishOutcome"
        Effect   = "Allow"
        Action   = "sns:Publish"
        Resource = module.messaging.topic_arn
      }
    ]
  })

  tags = local.tags
}

resource "kubernetes_service_account_v1" "payment_worker" {
  metadata {
    name      = "payment-worker"
    namespace = kubernetes_namespace_v1.stampede.metadata[0].name

    annotations = {
      "eks.amazonaws.com/role-arn" = module.payment_worker_irsa.role_arn
    }
  }
}

module "notification_worker_irsa" {
  source = "./modules/irsa"

  role_name            = "${var.project_name}-notification-worker-role"
  oidc_provider_arn    = module.eks_cluster.oidc_provider_arn
  oidc_provider_url    = module.eks_cluster.oidc_provider_url
  namespace            = "stampede"
  service_account_name = "notification-worker"

  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "AllowOwnQueueConsume"
        Effect   = "Allow"
        Action   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
        Resource = module.messaging.queue_arns["notification-worker"]
      }
    ]
  })

  tags = local.tags
}

resource "kubernetes_service_account_v1" "notification_worker" {
  metadata {
    name      = "notification-worker"
    namespace = kubernetes_namespace_v1.stampede.metadata[0].name

    annotations = {
      "eks.amazonaws.com/role-arn" = module.notification_worker_irsa.role_arn
    }
  }
}

module "order_service_irsa" {
  source = "./modules/irsa"

  role_name            = "${var.project_name}-order-service-role"
  oidc_provider_arn    = module.eks_cluster.oidc_provider_arn
  oidc_provider_url    = module.eks_cluster.oidc_provider_url
  namespace            = "stampede"
  service_account_name = "order-service"

  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "AllowOwnQueueConsume"
        Effect   = "Allow"
        Action   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
        Resource = module.messaging.queue_arns["order-service"]
      },
      {
        Sid      = "AllowPublishOrderPlaced"
        Effect   = "Allow"
        Action   = "sns:Publish"
        Resource = module.messaging.topic_arn
      },
      {
        Sid      = "AllowReadDbSecret"
        Effect   = "Allow"
        Action   = "secretsmanager:GetSecretValue"
        Resource = module.rds_orders.secret_arn
      }
    ]
  })

  tags = local.tags
}

resource "kubernetes_service_account_v1" "order_service" {
  metadata {
    name      = "order-service"
    namespace = kubernetes_namespace_v1.stampede.metadata[0].name

    annotations = {
      "eks.amazonaws.com/role-arn" = module.order_service_irsa.role_arn
    }
  }
}

module "catalog_service_irsa" {
  source = "./modules/irsa"

  role_name            = "${var.project_name}-catalog-service-role"
  oidc_provider_arn    = module.eks_cluster.oidc_provider_arn
  oidc_provider_url    = module.eks_cluster.oidc_provider_url
  namespace            = "stampede"
  service_account_name = "catalog-service"

  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "AllowReadDbSecret"
        Effect   = "Allow"
        Action   = "secretsmanager:GetSecretValue"
        Resource = module.rds_catalog.secret_arn
      }
    ]
  })

  tags = local.tags
}

resource "kubernetes_service_account_v1" "catalog_service" {
  metadata {
    name      = "catalog-service"
    namespace = kubernetes_namespace_v1.stampede.metadata[0].name

    annotations = {
      "eks.amazonaws.com/role-arn" = module.catalog_service_irsa.role_arn
    }
  }
}

resource "kubernetes_service_account_v1" "inventory_service" {
  metadata {
    name      = "inventory-service"
    namespace = kubernetes_namespace_v1.stampede.metadata[0].name
  }
}

resource "kubernetes_service_account_v1" "edge_gateway" {
  metadata {
    name      = "edge-gateway"
    namespace = kubernetes_namespace_v1.stampede.metadata[0].name
  }
}

module "ecr" {
  source = "./modules/ecr"

  project_name = var.project_name
  repository_names = [
    "edge-gateway",
    "catalog-service",
    "inventory-service",
    "order-service",
    "payment-worker",
    "fraud-detector",
    "notification-worker",
  ]

  tags = local.tags
}

module "codebuild" {
  source = "./modules/codebuild"

  project_name        = var.project_name
  aws_region          = "ap-south-1"
  ecr_repository_arns = module.ecr.repository_arns
  services = [
    "edge-gateway",
    "catalog-service",
    "inventory-service",
    "order-service",
    "payment-worker",
    "fraud-detector",
    "notification-worker",
  ]

  tags = local.tags
}

locals {
  environments = {
    dev = {
      node_desired_size   = 2
      node_min_size       = 2
      node_max_size       = 3
      node_instance_types = ["c7i-flex.large"]
      single_nat_gateway  = true

      rds_instance_class = "db.t3.micro"
      rds_multi_az       = false

      redis_node_type                  = "cache.t3.micro"
      redis_num_cache_clusters         = 1
      redis_automatic_failover_enabled = false

      opensearch_instance_type          = "t3.small.search"
      opensearch_instance_count         = 1
      opensearch_zone_awareness_enabled = false

      waf_rate_limit = 2000
    }

    staging = {
      node_desired_size   = 2
      node_min_size       = 1
      node_max_size       = 3
      node_instance_types = ["c7i-flex.xlarge"]
      single_nat_gateway  = true

      rds_instance_class = "db.t3.small"
      rds_multi_az       = false

      redis_node_type                  = "cache.t3.small"
      redis_num_cache_clusters         = 1
      redis_automatic_failover_enabled = false

      opensearch_instance_type          = "t3.small.search"
      opensearch_instance_count         = 1
      opensearch_zone_awareness_enabled = false

      waf_rate_limit = 2000
    }

    prod = {
      node_desired_size   = 3
      node_min_size       = 2
      node_max_size       = 6
      node_instance_types = ["c7i-flex.2xlarge"]
      single_nat_gateway  = false

      rds_instance_class = "db.t3.medium"
      rds_multi_az       = true

      redis_node_type                  = "cache.t3.medium"
      redis_num_cache_clusters         = 2
      redis_automatic_failover_enabled = true

      opensearch_instance_type          = "t3.medium.search"
      opensearch_instance_count         = 2
      opensearch_zone_awareness_enabled = true

      waf_rate_limit = 5000
    }
  }

  env = local.environments[terraform.workspace]
}

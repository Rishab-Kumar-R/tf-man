resource "local_file" "readme" {
  for_each = var.services
  filename = "${path.module}/output/${each.key}/README.md"
  content = templatefile("${path.module}/templates/readme.tftpl", {
    service_name = each.key
    owner        = var.owner
    port         = each.value.port
  })
}

resource "local_file" "dockerfile" {
  for_each = var.services
  filename = "${path.module}/output/${each.key}/Dockerfile"
  content = templatefile("${path.module}/templates/dockerfile.tftpl", {
    image = each.value.image
    port  = each.value.port
  })
}

resource "local_file" "env" {
  for_each = var.services
  filename = "${path.module}/output/${each.key}/.env"
  content = templatefile("${path.module}/templates/env.tftpl", {
    service_name = each.key
    port         = each.value.port
    replicas     = each.value.replicas
  })
}

resource "local_file" "deployment" {
  for_each = var.services
  filename = "${path.module}/output/${each.key}/deployment.yaml"
  content = templatefile("${path.module}/templates/deployments.tftpl", {
    service_name = each.key
    replicas     = each.value.replicas
    image        = each.value.image
    port         = each.value.port
  })
}

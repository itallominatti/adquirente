variable "name" { type = string }
variable "services" { type = list(string) }
variable "kms_key_arn" { type = string }

resource "aws_ecr_repository" "this" {
  for_each             = toset(var.services)
  name                 = "${var.name}/${each.value}"
  image_tag_mutability = "IMMUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "KMS"
    kms_key         = var.kms_key_arn
  }
}

# Mantém as 30 últimas imagens; apaga o resto. Imagem não usada é superfície de ataque e custo.
resource "aws_ecr_lifecycle_policy" "this" {
  for_each   = aws_ecr_repository.this
  repository = each.value.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "manter as 30 mais recentes"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 30
      }
      action = { type = "expire" }
    }]
  })
}

# Scan contínuo (não só no push): CVEs novas em imagens antigas aparecem no Inspector.
resource "aws_ecr_registry_scanning_configuration" "this" {
  scan_type = "ENHANCED"
  rule {
    scan_frequency = "CONTINUOUS_SCAN"
    repository_filter {
      filter      = "${var.name}/*"
      filter_type = "WILDCARD"
    }
  }
}

output "repository_urls" { value = { for k, r in aws_ecr_repository.this : k => r.repository_url } }

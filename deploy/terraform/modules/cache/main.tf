variable "name" { type = string }
variable "vpc_id" { type = string }
variable "subnet_ids" { type = list(string) }
variable "allowed_sg_ids" { type = list(string) }
variable "kms_key_arn" { type = string }

resource "aws_elasticache_subnet_group" "this" {
  name       = var.name
  subnet_ids = var.subnet_ids
}

resource "aws_security_group" "redis" {
  name        = "${var.name}-redis"
  description = "Redis: apenas a partir do cluster EKS"
  vpc_id      = var.vpc_id
  tags        = { Name = "${var.name}-redis" }
}

resource "aws_vpc_security_group_ingress_rule" "redis" {
  for_each                     = toset(var.allowed_sg_ids)
  security_group_id            = aws_security_group.redis.id
  referenced_security_group_id = each.value
  from_port                    = 6379
  to_port                      = 6379
  ip_protocol                  = "tcp"
}

# Senha forte gerada pelo Terraform e guardada SÓ no Secrets Manager (o state também a contém: por isso
# o state é cifrado e restrito).
resource "random_password" "auth" {
  length           = 64
  special          = true
  override_special = "!&#$^<>-" # caracteres aceitos pelo ElastiCache
}

resource "aws_secretsmanager_secret" "auth" {
  name       = "${var.name}/redis/auth-token"
  kms_key_id = var.kms_key_arn
}

resource "aws_secretsmanager_secret_version" "auth" {
  secret_id     = aws_secretsmanager_secret.auth.id
  secret_string = random_password.auth.result
}

resource "aws_elasticache_replication_group" "this" {
  replication_group_id = var.name
  description          = "Redis da adquirente (locks e cache)"
  engine               = "redis"
  engine_version       = "7.1"
  node_type            = "cache.t4g.small"
  port                 = 6379

  num_cache_clusters         = 2 # primário + réplica em outra AZ
  multi_az_enabled           = true
  automatic_failover_enabled = true

  subnet_group_name  = aws_elasticache_subnet_group.this.name
  security_group_ids = [aws_security_group.redis.id]

  at_rest_encryption_enabled = true
  kms_key_id                 = var.kms_key_arn
  transit_encryption_enabled = true
  auth_token                 = random_password.auth.result
  auth_token_update_strategy = "ROTATE"

  snapshot_retention_limit   = 7
  snapshot_window            = "02:00-03:00"
  maintenance_window         = "sun:05:00-sun:06:00"
  apply_immediately          = false
  auto_minor_version_upgrade = true
}

output "primary_endpoint" { value = aws_elasticache_replication_group.this.primary_endpoint_address }
output "replication_group_id" { value = aws_elasticache_replication_group.this.id }
output "auth_secret_arn" { value = aws_secretsmanager_secret.auth.arn }

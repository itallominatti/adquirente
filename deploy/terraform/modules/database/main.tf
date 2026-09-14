variable "name" { type = string }
variable "vpc_id" { type = string }
variable "subnet_ids" { type = list(string) }
variable "allowed_sg_ids" { type = list(string) }
variable "kms_key_arn" { type = string }
variable "performance_kms_arn" { type = string }
variable "instances" {
  type = map(object({
    instance_class    = string
    allocated_storage = number
  }))
}

resource "aws_db_subnet_group" "this" {
  name       = var.name
  subnet_ids = var.subnet_ids # subnets ISOLADAS: sem rota para a internet
}

# Security group: só a porta 5432, só a partir dos nós do EKS. Nenhum CIDR, nenhum 0.0.0.0/0.
resource "aws_security_group" "db" {
  name        = "${var.name}-rds"
  description = "PostgreSQL: apenas a partir do cluster EKS"
  vpc_id      = var.vpc_id
  tags        = { Name = "${var.name}-rds" }
}

resource "aws_vpc_security_group_ingress_rule" "db" {
  for_each                     = toset(var.allowed_sg_ids)
  security_group_id            = aws_security_group.db.id
  referenced_security_group_id = each.value
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
}

# Parâmetros do Postgres com foco em segurança e auditoria.
resource "aws_db_parameter_group" "this" {
  name   = "${var.name}-pg16"
  family = "postgres16"

  parameter { # recusa conexão sem TLS, mesmo de dentro da VPC
    name  = "rds.force_ssl"
    value = "1"
  }
  parameter { # pgaudit: quem executou qual comando (DDL e escritas)
    name         = "shared_preload_libraries"
    value        = "pgaudit"
    apply_method = "pending-reboot"
  }
  parameter {
    name  = "pgaudit.log"
    value = "ddl,write,role"
  }
  parameter { # consultas lentas (> 500 ms) no log: observabilidade
    name  = "log_min_duration_statement"
    value = "500"
  }
  parameter {
    name  = "log_connections"
    value = "1"
  }
  parameter {
    name  = "log_disconnections"
    value = "1"
  }
}

# Papel para o Enhanced Monitoring (métricas do SO a cada 60s).
resource "aws_iam_role" "monitoring" {
  name = "${var.name}-rds-monitoring"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "monitoring.rds.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "monitoring" {
  role       = aws_iam_role.monitoring.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}

resource "aws_db_instance" "this" {
  for_each = var.instances

  identifier     = "${var.name}-${each.key}"
  engine         = "postgres"
  engine_version = "16"
  instance_class = each.value.instance_class

  # armazenamento: gp3 cifrado, cresce sozinho até o teto
  allocated_storage     = each.value.allocated_storage
  max_allocated_storage = each.value.allocated_storage * 5
  storage_type          = "gp3"
  storage_encrypted     = true
  kms_key_id            = var.kms_key_arn

  db_name  = "adquirente"
  username = "adq_admin"
  # A AWS cria e ROTACIONA a senha no Secrets Manager. Ninguém conhece a senha; a aplicação lê o secret.
  manage_master_user_password   = true
  master_user_secret_kms_key_id = var.kms_key_arn

  # Autenticação por IAM: a aplicação pode se conectar com token IAM de 15 min em vez de senha.
  iam_database_authentication_enabled = true

  # rede
  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.db.id]
  publicly_accessible    = false
  multi_az               = true
  parameter_group_name   = aws_db_parameter_group.this.name

  # backups e proteção
  backup_retention_period   = 35            # PITR: voltar para qualquer segundo dos últimos 35 dias
  backup_window             = "03:00-04:00" # UTC (00:00-01:00 Brasília): fora da liquidação
  maintenance_window        = "sun:04:00-sun:05:00"
  copy_tags_to_snapshot     = true
  deletion_protection       = true
  skip_final_snapshot       = false
  final_snapshot_identifier = "${var.name}-${each.key}-final"
  delete_automated_backups  = false

  # observabilidade
  performance_insights_enabled          = true
  performance_insights_kms_key_id       = var.performance_kms_arn
  performance_insights_retention_period = 7
  monitoring_interval                   = 60
  monitoring_role_arn                   = aws_iam_role.monitoring.arn
  enabled_cloudwatch_logs_exports       = ["postgresql", "upgrade"]

  # atualizações
  auto_minor_version_upgrade  = true
  allow_major_version_upgrade = false
  apply_immediately           = false

  tags = { Component = each.key }
}

output "identifiers" { value = { for k, db in aws_db_instance.this : k => db.identifier } }
output "endpoints" { value = { for k, db in aws_db_instance.this : k => db.endpoint } }
output "secret_arn" { value = { for k, db in aws_db_instance.this : k => db.master_user_secret[0].secret_arn } }
output "security_group_id" { value = aws_security_group.db.id }

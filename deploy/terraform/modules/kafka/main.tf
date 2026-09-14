variable "name" { type = string }
variable "vpc_id" { type = string }
variable "subnet_ids" { type = list(string) }
variable "allowed_sg_ids" { type = list(string) }
variable "kms_key_arn" { type = string }
variable "log_kms_arn" { type = string }

resource "aws_security_group" "kafka" {
  name        = "${var.name}-msk"
  description = "Kafka: apenas a partir do cluster EKS"
  vpc_id      = var.vpc_id
  tags        = { Name = "${var.name}-msk" }
}

resource "aws_vpc_security_group_ingress_rule" "kafka_iam" {
  for_each                     = toset(var.allowed_sg_ids)
  security_group_id            = aws_security_group.kafka.id
  referenced_security_group_id = each.value
  from_port                    = 9098 # porta do SASL/IAM com TLS
  to_port                      = 9098
  ip_protocol                  = "tcp"
}

# Configuração do broker: durabilidade de dados de pagamento acima de tudo.
resource "aws_msk_configuration" "this" {
  name              = var.name
  kafka_versions    = ["3.6.0"]
  server_properties = <<-PROPERTIES
    auto.create.topics.enable=false
    default.replication.factor=3
    min.insync.replicas=2
    unclean.leader.election.enable=false
    num.partitions=6
    log.retention.hours=168
  PROPERTIES
}

resource "aws_cloudwatch_log_group" "kafka" {
  name              = "/aws/msk/${var.name}"
  retention_in_days = 90
  kms_key_id        = var.log_kms_arn
}

resource "aws_msk_cluster" "this" {
  cluster_name           = var.name
  kafka_version          = "3.6.0"
  number_of_broker_nodes = 3

  broker_node_group_info {
    instance_type   = "kafka.m7g.large"
    client_subnets  = var.subnet_ids
    security_groups = [aws_security_group.kafka.id]
    storage_info {
      ebs_storage_info { volume_size = 200 }
    }
    connectivity_info {
      public_access { type = "DISABLED" } # brokers nunca na internet
    }
  }

  configuration_info {
    arn      = aws_msk_configuration.this.arn
    revision = aws_msk_configuration.this.latest_revision
  }

  encryption_info {
    encryption_at_rest_kms_key_arn = var.kms_key_arn
    encryption_in_transit {
      client_broker = "TLS"
      in_cluster    = true
    }
  }

  client_authentication {
    sasl { iam = true } # cada pod autentica com sua role (Pod Identity); nada de usuário/senha
    unauthenticated = false
  }

  enhanced_monitoring = "PER_TOPIC_PER_PARTITION"

  logging_info {
    broker_logs {
      cloudwatch_logs {
        enabled   = true
        log_group = aws_cloudwatch_log_group.kafka.name
      }
    }
  }

  open_monitoring {
    prometheus {
      jmx_exporter { enabled_in_broker = true }
      node_exporter { enabled_in_broker = true }
    }
  }
}

output "cluster_name" { value = aws_msk_cluster.this.cluster_name }
output "cluster_arn" { value = aws_msk_cluster.this.arn }
output "bootstrap_brokers_sasl_iam" { value = aws_msk_cluster.this.bootstrap_brokers_sasl_iam }

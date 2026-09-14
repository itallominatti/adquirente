variable "name" { type = string }

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# ---------- KMS: uma chave por finalidade (separação de responsabilidades) ----------
# Se a chave de logs vazar, os dados continuam protegidos; e o acesso a cada chave é auditado separadamente.

resource "aws_kms_key" "data" {
  description             = "${var.name}: dados (RDS, S3, MSK, ElastiCache)"
  enable_key_rotation     = true
  deletion_window_in_days = 30
}
resource "aws_kms_alias" "data" {
  name          = "alias/${var.name}-data"
  target_key_id = aws_kms_key.data.key_id
}

resource "aws_kms_key" "eks" {
  description             = "${var.name}: Secrets do Kubernetes (envelope encryption no etcd)"
  enable_key_rotation     = true
  deletion_window_in_days = 30
}
resource "aws_kms_alias" "eks" {
  name          = "alias/${var.name}-eks"
  target_key_id = aws_kms_key.eks.key_id
}

# A chave de logs precisa permitir que o serviço CloudWatch Logs a use.
data "aws_iam_policy_document" "kms_logs" {
  statement {
    sid       = "Root"
    actions   = ["kms:*"]
    resources = ["*"]
    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"]
    }
  }
  statement {
    sid       = "CloudWatchLogs"
    actions   = ["kms:Encrypt*", "kms:Decrypt*", "kms:ReEncrypt*", "kms:GenerateDataKey*", "kms:Describe*"]
    resources = ["*"]
    principals {
      type        = "Service"
      identifiers = ["logs.${data.aws_region.current.region}.amazonaws.com", "cloudtrail.amazonaws.com", "delivery.logs.amazonaws.com"]
    }
  }
}

resource "aws_kms_key" "logs" {
  description             = "${var.name}: logs (CloudWatch, CloudTrail, Flow Logs, WAF)"
  enable_key_rotation     = true
  deletion_window_in_days = 30
  policy                  = data.aws_iam_policy_document.kms_logs.json
}
resource "aws_kms_alias" "logs" {
  name          = "alias/${var.name}-logs"
  target_key_id = aws_kms_key.logs.key_id
}

# ---------- Travas de conta ----------

# Todo volume EBS novo nasce cifrado, mesmo que alguém esqueça.
resource "aws_ebs_encryption_by_default" "this" { enabled = true }

# Nenhum bucket da conta pode virar público, mesmo que alguém tente.
resource "aws_s3_account_public_access_block" "this" {
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Senhas de usuários IAM (os poucos que existirem; o certo é SSO): fortes e rotacionadas.
resource "aws_iam_account_password_policy" "this" {
  minimum_password_length        = 14
  require_lowercase_characters   = true
  require_uppercase_characters   = true
  require_numbers                = true
  require_symbols                = true
  max_password_age               = 90
  password_reuse_prevention      = 24
  allow_users_to_change_password = true
}

# ---------- CloudTrail: QUEM fez O QUÊ, QUANDO, DE ONDE, em todas as regiões ----------

resource "aws_s3_bucket" "trail" {
  bucket        = "${var.name}-cloudtrail-${data.aws_caller_identity.current.account_id}"
  force_destroy = false
}

resource "aws_s3_bucket_versioning" "trail" {
  bucket = aws_s3_bucket.trail.id
  versioning_configuration { status = "Enabled" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "trail" {
  bucket = aws_s3_bucket.trail.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = aws_kms_key.logs.arn
    }
  }
}

resource "aws_s3_bucket_public_access_block" "trail" {
  bucket                  = aws_s3_bucket.trail.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Object Lock em modo COMPLIANCE: nem o root apaga um log de auditoria antes do prazo.
resource "aws_s3_bucket_object_lock_configuration" "trail" {
  bucket = aws_s3_bucket.trail.id
  rule {
    default_retention {
      mode  = "COMPLIANCE"
      years = 1 # PCI DSS: logs de auditoria por pelo menos 1 ano
    }
  }
  depends_on = [aws_s3_bucket_versioning.trail]
}

data "aws_iam_policy_document" "trail_bucket" {
  statement {
    sid       = "AWSCloudTrailAclCheck"
    actions   = ["s3:GetBucketAcl"]
    resources = [aws_s3_bucket.trail.arn]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
  }
  statement {
    sid       = "AWSCloudTrailWrite"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.trail.arn}/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "s3:x-amz-acl"
      values   = ["bucket-owner-full-control"]
    }
  }
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    resources = [aws_s3_bucket.trail.arn, "${aws_s3_bucket.trail.arn}/*"]
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "trail" {
  bucket = aws_s3_bucket.trail.id
  policy = data.aws_iam_policy_document.trail_bucket.json
}

# Além do S3 (arquivo imutável), o CloudTrail também envia para o CloudWatch Logs: é o que permite
# alarmes em tempo quase real ("alguém usou o root", "alguém desligou o GuardDuty").
resource "aws_cloudwatch_log_group" "trail" {
  name              = "/aws/cloudtrail/${var.name}"
  retention_in_days = 400
  kms_key_id        = aws_kms_key.logs.arn
}

resource "aws_iam_role" "trail" {
  name = "${var.name}-cloudtrail-logs"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "cloudtrail.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy" "trail" {
  role = aws_iam_role.trail.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
      Resource = "${aws_cloudwatch_log_group.trail.arn}:*"
    }]
  })
}

resource "aws_cloudtrail" "this" {
  name                          = var.name
  s3_bucket_name                = aws_s3_bucket.trail.id
  cloud_watch_logs_group_arn    = "${aws_cloudwatch_log_group.trail.arn}:*"
  cloud_watch_logs_role_arn     = aws_iam_role.trail.arn
  is_multi_region_trail         = true # um atacante que cria recursos em outra região também aparece
  include_global_service_events = true # IAM, STS, Route53
  enable_log_file_validation    = true # hash encadeado: prova que ninguém alterou os logs
  kms_key_id                    = aws_kms_key.logs.arn
  depends_on                    = [aws_s3_bucket_policy.trail]
}

# ---------- GuardDuty: detecção de ameaças (credencial vazada, mineração, exfiltração) ----------
resource "aws_guardduty_detector" "this" {
  enable = true
}

# ---------- AWS Config: registra o estado de cada recurso e avalia regras continuamente ----------
resource "aws_iam_role" "config" {
  name = "${var.name}-config"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "config.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "config" {
  role       = aws_iam_role.config.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWS_ConfigRole"
}

resource "aws_config_configuration_recorder" "this" {
  name     = var.name
  role_arn = aws_iam_role.config.arn
  recording_group {
    all_supported                 = true
    include_global_resource_types = true
  }
}

resource "aws_config_delivery_channel" "this" {
  name           = var.name
  s3_bucket_name = aws_s3_bucket.trail.id
  s3_key_prefix  = "config"
  depends_on     = [aws_config_configuration_recorder.this]
}

resource "aws_config_configuration_recorder_status" "this" {
  name       = aws_config_configuration_recorder.this.name
  is_enabled = true
  depends_on = [aws_config_delivery_channel.this]
}

# Regras gerenciadas: cada uma é uma pergunta que a AWS faz o tempo todo aos seus recursos.
locals {
  config_rules = {
    "rds-storage-encrypted"                  = "RDS_STORAGE_ENCRYPTED"
    "rds-instance-public-access-check"       = "RDS_INSTANCE_PUBLIC_ACCESS_CHECK"
    "s3-bucket-public-read-prohibited"       = "S3_BUCKET_PUBLIC_READ_PROHIBITED"
    "s3-bucket-server-side-encryption"       = "S3_BUCKET_SERVER_SIDE_ENCRYPTION_ENABLED"
    "encrypted-volumes"                      = "ENCRYPTED_VOLUMES"
    "iam-root-access-key-check"              = "IAM_ROOT_ACCESS_KEY_CHECK"
    "root-account-mfa-enabled"               = "ROOT_ACCOUNT_MFA_ENABLED"
    "cloud-trail-encryption-enabled"         = "CLOUD_TRAIL_ENCRYPTION_ENABLED"
    "eks-endpoint-no-public-access"          = "EKS_ENDPOINT_NO_PUBLIC_ACCESS"
    "elasticache-repl-grp-encrypted-at-rest" = "ELASTICACHE_REPL_GRP_ENCRYPTED_AT_REST"
  }
}

resource "aws_config_config_rule" "managed" {
  for_each = local.config_rules
  name     = each.key
  source {
    owner             = "AWS"
    source_identifier = each.value
  }
  depends_on = [aws_config_configuration_recorder_status.this]
}

# ---------- Security Hub: agrega GuardDuty, Config, Inspector num painel e compara com benchmarks ----------
resource "aws_securityhub_account" "this" {}

resource "aws_securityhub_standards_subscription" "aws_foundational" {
  standards_arn = "arn:aws:securityhub:${data.aws_region.current.region}::standards/aws-foundational-security-best-practices/v/1.0.0"
  depends_on    = [aws_securityhub_account.this]
}

resource "aws_securityhub_standards_subscription" "pci" {
  standards_arn = "arn:aws:securityhub:${data.aws_region.current.region}::standards/pci-dss/v/3.2.1"
  depends_on    = [aws_securityhub_account.this]
}

# ---------- IAM Access Analyzer: quem de FORA da conta consegue acessar o quê ----------
resource "aws_accessanalyzer_analyzer" "this" {
  analyzer_name = var.name
  type          = "ACCOUNT"
}

output "kms_data_arn" { value = aws_kms_key.data.arn }
output "kms_eks_arn" { value = aws_kms_key.eks.arn }
output "kms_logs_arn" { value = aws_kms_key.logs.arn }
output "trail_bucket" { value = aws_s3_bucket.trail.bucket }
output "trail_log_group" { value = aws_cloudwatch_log_group.trail.name }

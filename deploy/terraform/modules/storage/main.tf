variable "name" { type = string }
variable "kms_key_arn" { type = string }

data "aws_caller_identity" "current" {}
data "aws_elb_service_account" "this" {}

# ---------- arquivos de liquidação ----------
resource "aws_s3_bucket" "settlement" {
  bucket              = "${var.name}-settlement-files-${data.aws_caller_identity.current.account_id}"
  object_lock_enabled = true # precisa ser decidido na criação; não dá para ligar depois
}

resource "aws_s3_bucket_versioning" "settlement" {
  bucket = aws_s3_bucket.settlement.id
  versioning_configuration { status = "Enabled" }
}

# COMPLIANCE + 5 anos: obrigação regulatória de guardar o histórico. Ninguém apaga antes, nem o root.
resource "aws_s3_bucket_object_lock_configuration" "settlement" {
  bucket = aws_s3_bucket.settlement.id
  rule {
    default_retention {
      mode  = "COMPLIANCE"
      years = 5
    }
  }
  depends_on = [aws_s3_bucket_versioning.settlement]
}

resource "aws_s3_bucket_server_side_encryption_configuration" "settlement" {
  bucket = aws_s3_bucket.settlement.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = var.kms_key_arn
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_public_access_block" "settlement" {
  bucket                  = aws_s3_bucket.settlement.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Depois de 90 dias o arquivo raramente é lido: vai para uma classe mais barata, sem perder o Object Lock.
resource "aws_s3_bucket_lifecycle_configuration" "settlement" {
  bucket = aws_s3_bucket.settlement.id
  rule {
    id     = "archive"
    status = "Enabled"
    filter {}
    transition {
      days          = 90
      storage_class = "GLACIER_IR"
    }
    noncurrent_version_transition {
      noncurrent_days = 30
      storage_class   = "GLACIER_IR"
    }
  }
}

# Registro de quem acessou cada arquivo (PCI: auditar acesso a dados financeiros).
resource "aws_s3_bucket_logging" "settlement" {
  bucket        = aws_s3_bucket.settlement.id
  target_bucket = aws_s3_bucket.access_logs.id
  target_prefix = "settlement/"
}

data "aws_iam_policy_document" "settlement" {
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    resources = [aws_s3_bucket.settlement.arn, "${aws_s3_bucket.settlement.arn}/*"]
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
  # Recusa uploads que não peçam cifragem com a NOSSA chave.
  statement {
    sid     = "DenyWrongEncryption"
    effect  = "Deny"
    actions = ["s3:PutObject"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    resources = ["${aws_s3_bucket.settlement.arn}/*"]
    condition {
      test     = "StringNotEquals"
      variable = "s3:x-amz-server-side-encryption-aws-kms-key-id"
      values   = [var.kms_key_arn]
    }
  }
}

resource "aws_s3_bucket_policy" "settlement" {
  bucket = aws_s3_bucket.settlement.id
  policy = data.aws_iam_policy_document.settlement.json
}

# ---------- logs de acesso (S3 e ALB) ----------
resource "aws_s3_bucket" "access_logs" {
  bucket = "${var.name}-access-logs-${data.aws_caller_identity.current.account_id}"
}

resource "aws_s3_bucket_versioning" "access_logs" {
  bucket = aws_s3_bucket.access_logs.id
  versioning_configuration { status = "Enabled" }
}

# Logs do ALB só aceitam SSE-S3 (AES256), não KMS: limitação do serviço.
resource "aws_s3_bucket_server_side_encryption_configuration" "access_logs" {
  bucket = aws_s3_bucket.access_logs.id
  rule {
    apply_server_side_encryption_by_default { sse_algorithm = "AES256" }
  }
}

resource "aws_s3_bucket_public_access_block" "access_logs" {
  bucket                  = aws_s3_bucket.access_logs.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_lifecycle_configuration" "access_logs" {
  bucket = aws_s3_bucket.access_logs.id
  rule {
    id     = "expire"
    status = "Enabled"
    filter {}
    expiration { days = 400 } # PCI pede 1 ano; damos folga
  }
}

data "aws_iam_policy_document" "access_logs" {
  # O serviço de ALB da região escreve os logs de acesso aqui.
  statement {
    sid       = "ALBLogDelivery"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.access_logs.arn}/alb/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]
    principals {
      type        = "AWS"
      identifiers = [data.aws_elb_service_account.this.arn]
    }
  }
  statement {
    sid       = "S3ServerAccessLogs"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.access_logs.arn}/*"]
    principals {
      type        = "Service"
      identifiers = ["logging.s3.amazonaws.com"]
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
    resources = [aws_s3_bucket.access_logs.arn, "${aws_s3_bucket.access_logs.arn}/*"]
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "access_logs" {
  bucket = aws_s3_bucket.access_logs.id
  policy = data.aws_iam_policy_document.access_logs.json
}

output "settlement_bucket_name" { value = aws_s3_bucket.settlement.bucket }
output "settlement_bucket_arn" { value = aws_s3_bucket.settlement.arn }
output "access_logs_bucket_name" { value = aws_s3_bucket.access_logs.bucket }

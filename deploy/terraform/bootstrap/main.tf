terraform {
  required_version = ">= 1.10"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  region = var.region
  default_tags {
    tags = { Project = "adquirente", ManagedBy = "terraform", Component = "bootstrap" }
  }
}

variable "region" {
  type    = string
  default = "sa-east-1" # São Paulo: dados de pagamento brasileiros ficam no Brasil
}

variable "state_bucket_name" {
  type        = string
  description = "Nome globalmente único do bucket do state, ex.: adquirente-prod-tfstate-123456789012"
}

# Chave KMS própria para o state: quem pode ler o state é quem pode usar esta chave.
resource "aws_kms_key" "tfstate" {
  description             = "Cifra o Terraform state"
  enable_key_rotation     = true # a AWS troca o material da chave todo ano, sem você fazer nada
  deletion_window_in_days = 30
}

resource "aws_kms_alias" "tfstate" {
  name          = "alias/adquirente-tfstate"
  target_key_id = aws_kms_key.tfstate.key_id
}

resource "aws_s3_bucket" "tfstate" {
  bucket = var.state_bucket_name
  lifecycle {
    prevent_destroy = true # um terraform destroy acidental não pode apagar o state
  }
}

# Versionamento: todo apply gera uma versão nova; dá para voltar se alguém corromper o state.
resource "aws_s3_bucket_versioning" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  versioning_configuration { status = "Enabled" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = aws_kms_key.tfstate.arn
    }
    bucket_key_enabled = true
  }
}

# Bloqueio TOTAL de acesso público. Vale para qualquer bucket deste projeto.
resource "aws_s3_bucket_public_access_block" "tfstate" {
  bucket                  = aws_s3_bucket.tfstate.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Política: recusa qualquer acesso que não seja por TLS.
data "aws_iam_policy_document" "tfstate" {
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    resources = [aws_s3_bucket.tfstate.arn, "${aws_s3_bucket.tfstate.arn}/*"]
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  policy = data.aws_iam_policy_document.tfstate.json
}

output "state_bucket" { value = aws_s3_bucket.tfstate.bucket }
output "state_kms_key_arn" { value = aws_kms_key.tfstate.arn }

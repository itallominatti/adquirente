terraform {
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # O state fica no bucket criado pelo bootstrap. use_lockfile (Terraform >= 1.10) usa o próprio S3
  # para o lock: dois applies ao mesmo tempo não se atropelam.
  backend "s3" {
    bucket       = "adquirente-prod-tfstate-SUBSTITUA"
    key          = "envs/prod/terraform.tfstate"
    region       = "sa-east-1"
    encrypt      = true
    kms_key_id   = "alias/adquirente-tfstate"
    use_lockfile = true
  }
}

provider "aws" {
  region = var.region

  # Tags em TUDO: custo por componente, dono, ambiente. Auditoria e FinOps agradecem.
  default_tags {
    tags = {
      Project     = "adquirente"
      Environment = var.environment
      ManagedBy   = "terraform"
      DataClass   = "payments" # sinaliza dados de pagamento para políticas de compliance
    }
  }
}

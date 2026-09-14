locals {
  name = "adquirente-${var.environment}"
  azs  = ["${var.region}a", "${var.region}b", "${var.region}c"] # 3 zonas: uma cai, duas continuam
}

module "security" {
  source = "../../modules/security"
  name   = local.name
}

module "network" {
  source           = "../../modules/network"
  name             = local.name
  cidr             = var.vpc_cidr
  azs              = local.azs
  flow_log_kms_arn = module.security.kms_logs_arn
}

module "storage" {
  source      = "../../modules/storage"
  name        = local.name
  kms_key_arn = module.security.kms_data_arn
}

module "registry" {
  source      = "../../modules/registry"
  name        = local.name
  services    = ["api", "vault", "issuer-sim", "scheduler", "settlement", "notifier"]
  kms_key_arn = module.security.kms_data_arn
}

module "eks" {
  source             = "../../modules/eks"
  name               = local.name
  cluster_version    = var.eks_version
  vpc_id             = module.network.vpc_id
  private_subnet_ids = module.network.private_subnet_ids
  kms_secrets_arn    = module.security.kms_eks_arn
  log_kms_arn        = module.security.kms_logs_arn
  secrets_arns       = [module.database.secret_arn["core"], module.database.secret_arn["vault"], module.cache.auth_secret_arn]
  settlement_bucket  = module.storage.settlement_bucket_arn
}

module "database" {
  source              = "../../modules/database"
  name                = local.name
  vpc_id              = module.network.vpc_id
  subnet_ids          = module.network.isolated_subnet_ids
  allowed_sg_ids      = [module.eks.node_security_group_id]
  kms_key_arn         = module.security.kms_data_arn
  performance_kms_arn = module.security.kms_data_arn
  instances = {
    core  = { instance_class = "db.r6g.large", allocated_storage = 100 }
    vault = { instance_class = "db.t4g.medium", allocated_storage = 20 } # banco separado para o cofre de cartões
  }
}

module "cache" {
  source         = "../../modules/cache"
  name           = local.name
  vpc_id         = module.network.vpc_id
  subnet_ids     = module.network.isolated_subnet_ids
  allowed_sg_ids = [module.eks.node_security_group_id]
  kms_key_arn    = module.security.kms_data_arn
}

module "kafka" {
  source         = "../../modules/kafka"
  name           = local.name
  vpc_id         = module.network.vpc_id
  subnet_ids     = module.network.private_subnet_ids
  allowed_sg_ids = [module.eks.node_security_group_id]
  kms_key_arn    = module.security.kms_data_arn
  log_kms_arn    = module.security.kms_logs_arn
}

module "edge" {
  source            = "../../modules/edge"
  name              = local.name
  domain_name       = var.domain_name
  route53_zone_name = var.route53_zone_name
  log_kms_arn       = module.security.kms_logs_arn
}

module "observability" {
  source               = "../../modules/observability"
  name                 = local.name
  alert_email          = var.alert_email
  kms_key_arn          = module.security.kms_logs_arn
  db_identifiers       = module.database.identifiers
  msk_cluster_name     = module.kafka.cluster_name
  redis_group_id       = module.cache.replication_group_id
  monthly_budget_usd   = var.monthly_budget_usd
  cloudtrail_log_group = module.security.trail_log_group
}

variable "region" {
  type    = string
  default = "sa-east-1"
}

variable "environment" {
  type    = string
  default = "prod"
}

variable "vpc_cidr" {
  type    = string
  default = "10.40.0.0/16"
}

variable "domain_name" {
  type        = string
  description = "Domínio da API, ex.: api.adquirente.com (a zona Route53 já deve existir)"
}

variable "route53_zone_name" {
  type        = string
  description = "Nome da zona hospedada, ex.: adquirente.com"
}

variable "alert_email" {
  type        = string
  description = "E-mail que recebe os alarmes (confirme a inscrição do SNS)"
}

variable "eks_version" {
  type    = string
  default = "1.31"
}

variable "monthly_budget_usd" {
  type    = number
  default = 3000
}

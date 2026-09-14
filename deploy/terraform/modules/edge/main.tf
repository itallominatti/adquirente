variable "name" { type = string }
variable "domain_name" { type = string }
variable "route53_zone_name" { type = string }
variable "log_kms_arn" { type = string }

data "aws_route53_zone" "this" {
  name = var.route53_zone_name
}

# ---------- certificado TLS ----------
resource "aws_acm_certificate" "api" {
  domain_name       = var.domain_name
  validation_method = "DNS"
  lifecycle { create_before_destroy = true }
}

# A validação por DNS prova que somos dono do domínio; a ACM renova sozinha antes de expirar.
resource "aws_route53_record" "validation" {
  for_each = {
    for dvo in aws_acm_certificate.api.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }
  zone_id = data.aws_route53_zone.this.zone_id
  name    = each.value.name
  type    = each.value.type
  ttl     = 60
  records = [each.value.record]
}

resource "aws_acm_certificate_validation" "api" {
  certificate_arn         = aws_acm_certificate.api.arn
  validation_record_fqdns = [for r in aws_route53_record.validation : r.fqdn]
}

# ---------- WAF ----------
resource "aws_wafv2_web_acl" "api" {
  name  = "${var.name}-api"
  scope = "REGIONAL" # para ALB (CLOUDFRONT seria para CDN)

  default_action {
    allow {}
  }

  # Regras gerenciadas pela AWS: assinaturas de ataques atualizadas por eles.
  dynamic "rule" {
    for_each = {
      AWSManagedRulesCommonRuleSet          = 10
      AWSManagedRulesKnownBadInputsRuleSet  = 20
      AWSManagedRulesSQLiRuleSet            = 30
      AWSManagedRulesAmazonIpReputationList = 40
    }
    content {
      name     = rule.key
      priority = rule.value
      override_action {
        none {}
      }
      statement {
        managed_rule_group_statement {
          name        = rule.key
          vendor_name = "AWS"
        }
      }
      visibility_config {
        cloudwatch_metrics_enabled = true
        metric_name                = rule.key
        sampled_requests_enabled   = true
      }
    }
  }

  # Rate limit: mais de 2.000 requisições em 5 min do mesmo IP → bloqueia. Freia abuso e credential stuffing.
  rule {
    name     = "rate-limit-per-ip"
    priority = 50
    action {
      block {}
    }
    statement {
      rate_based_statement {
        limit              = 2000
        aggregate_key_type = "IP"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "rate-limit-per-ip"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${var.name}-api"
    sampled_requests_enabled   = true
  }
}

# Logs do WAF: o nome do log group PRECISA começar com "aws-waf-logs-".
resource "aws_cloudwatch_log_group" "waf" {
  name              = "aws-waf-logs-${var.name}"
  retention_in_days = 365
  kms_key_id        = var.log_kms_arn
}

resource "aws_wafv2_web_acl_logging_configuration" "api" {
  resource_arn            = aws_wafv2_web_acl.api.arn
  log_destination_configs = [aws_cloudwatch_log_group.waf.arn]

  # Não grava o header Authorization nos logs: token não pode aparecer em log.
  redacted_fields {
    single_header { name = "authorization" }
  }
}

output "certificate_arn" { value = aws_acm_certificate_validation.api.certificate_arn }
output "web_acl_arn" { value = aws_wafv2_web_acl.api.arn }
output "zone_id" { value = data.aws_route53_zone.this.zone_id }

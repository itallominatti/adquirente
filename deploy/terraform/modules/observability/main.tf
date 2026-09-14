variable "name" { type = string }
variable "alert_email" { type = string }
variable "kms_key_arn" { type = string }
variable "db_identifiers" { type = map(string) }
variable "msk_cluster_name" { type = string }
variable "redis_group_id" { type = string }
variable "monthly_budget_usd" { type = number }
variable "cloudtrail_log_group" { type = string }

# Tópico SNS cifrado: todo alarme publica aqui; e-mail, Slack, PagerDuty se inscrevem.
resource "aws_sns_topic" "alerts" {
  name              = "${var.name}-alerts"
  kms_master_key_id = var.kms_key_arn
}

resource "aws_sns_topic_subscription" "email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

# ---------- RDS ----------
resource "aws_cloudwatch_metric_alarm" "rds_cpu" {
  for_each            = var.db_identifiers
  alarm_name          = "${each.value}-cpu-high"
  namespace           = "AWS/RDS"
  metric_name         = "CPUUtilization"
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 3
  threshold           = 80
  comparison_operator = "GreaterThanThreshold"
  dimensions          = { DBInstanceIdentifier = each.value }
  alarm_actions       = [aws_sns_topic.alerts.arn]
  ok_actions          = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "rds_storage" {
  for_each            = var.db_identifiers
  alarm_name          = "${each.value}-free-storage-low"
  namespace           = "AWS/RDS"
  metric_name         = "FreeStorageSpace"
  statistic           = "Minimum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 10 * 1024 * 1024 * 1024 # 10 GiB
  comparison_operator = "LessThanThreshold"
  dimensions          = { DBInstanceIdentifier = each.value }
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "rds_connections" {
  for_each            = var.db_identifiers
  alarm_name          = "${each.value}-connections-high"
  namespace           = "AWS/RDS"
  metric_name         = "DatabaseConnections"
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 5
  threshold           = 400
  comparison_operator = "GreaterThanThreshold"
  dimensions          = { DBInstanceIdentifier = each.value }
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

# ---------- MSK ----------
resource "aws_cloudwatch_metric_alarm" "msk_offline_partitions" {
  alarm_name          = "${var.name}-msk-offline-partitions"
  namespace           = "AWS/Kafka"
  metric_name         = "OfflinePartitionsCount"
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  dimensions          = { "Cluster Name" = var.msk_cluster_name }
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "msk_disk" {
  alarm_name          = "${var.name}-msk-disk-high"
  namespace           = "AWS/Kafka"
  metric_name         = "KafkaDataLogsDiskUsed"
  statistic           = "Maximum"
  period              = 300
  evaluation_periods  = 2
  threshold           = 80
  comparison_operator = "GreaterThanThreshold"
  dimensions          = { "Cluster Name" = var.msk_cluster_name }
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

# ---------- Redis ----------
resource "aws_cloudwatch_metric_alarm" "redis_memory" {
  alarm_name          = "${var.name}-redis-memory-high"
  namespace           = "AWS/ElastiCache"
  metric_name         = "DatabaseMemoryUsagePercentage"
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 2
  threshold           = 80
  comparison_operator = "GreaterThanThreshold"
  dimensions          = { ReplicationGroupId = var.redis_group_id }
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

# ---------- segurança operacional ----------
# Uso do usuário root = incidente. Alarme via métrica de log do CloudTrail.
resource "aws_cloudwatch_log_metric_filter" "root_usage" {
  name           = "${var.name}-root-usage"
  log_group_name = var.cloudtrail_log_group
  pattern        = "{ $.userIdentity.type = \"Root\" && $.userIdentity.invokedBy NOT EXISTS && $.eventType != \"AwsServiceEvent\" }"
  metric_transformation {
    name      = "RootUsage"
    namespace = var.name
    value     = "1"
  }
}

resource "aws_cloudwatch_metric_alarm" "root_usage" {
  alarm_name          = "${var.name}-root-usage"
  namespace           = var.name
  metric_name         = "RootUsage"
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  treat_missing_data  = "notBreaching"
}

# ---------- custo ----------
resource "aws_budgets_budget" "monthly" {
  name         = "${var.name}-monthly"
  budget_type  = "COST"
  limit_amount = tostring(var.monthly_budget_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 80
    threshold_type             = "PERCENTAGE"
    notification_type          = "FORECASTED"
    subscriber_email_addresses = [var.alert_email]
  }
}

output "alerts_topic_arn" { value = aws_sns_topic.alerts.arn }

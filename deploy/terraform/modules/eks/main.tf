variable "name" { type = string }
variable "cluster_version" { type = string }
variable "vpc_id" { type = string }
variable "private_subnet_ids" { type = list(string) }
variable "kms_secrets_arn" { type = string }
variable "log_kms_arn" { type = string }
variable "secrets_arns" { type = list(string) }
variable "settlement_bucket" { type = string }

data "aws_caller_identity" "current" {}

# ---------- papel do control plane ----------
resource "aws_iam_role" "cluster" {
  name = "${var.name}-eks-cluster"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "eks.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "cluster" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

# Logs do control plane (api, audit, authenticator...): quem chamou o quê na API do Kubernetes.
resource "aws_cloudwatch_log_group" "cluster" {
  name              = "/aws/eks/${var.name}/cluster"
  retention_in_days = 365
  kms_key_id        = var.log_kms_arn
}

resource "aws_eks_cluster" "this" {
  name     = var.name
  version  = var.cluster_version
  role_arn = aws_iam_role.cluster.arn

  vpc_config {
    subnet_ids              = var.private_subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = false # kubectl só de dentro da VPC (VPN/bastion/SSM). A API do cluster não existe na internet.
  }

  # Secrets do Kubernetes ficam no etcd; aqui garantimos que estão cifrados com a NOSSA chave.
  encryption_config {
    provider { key_arn = var.kms_secrets_arn }
    resources = ["secrets"]
  }

  access_config {
    authentication_mode                         = "API" # acesso gerido por access entries (IAM), auditável
    bootstrap_cluster_creator_admin_permissions = true
  }

  enabled_cluster_log_types = ["api", "audit", "authenticator", "controllerManager", "scheduler"]

  depends_on = [aws_iam_role_policy_attachment.cluster, aws_cloudwatch_log_group.cluster]
}

# ---------- nós ----------
resource "aws_iam_role" "node" {
  name = "${var.name}-eks-node"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "ec2.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "node" {
  for_each = toset([
    "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
    "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore", # acesso ao nó via SSM Session Manager: sem SSH, sem porta 22
  ])
  role       = aws_iam_role.node.name
  policy_arn = each.value
}

# Launch template para exigir IMDSv2: fecha o ataque clássico de roubar credenciais do nó via SSRF.
resource "aws_launch_template" "node" {
  name_prefix = "${var.name}-node-"
  metadata_options {
    http_tokens                 = "required"
    http_put_response_hop_limit = 1 # pods não alcançam o metadata do nó
    http_endpoint               = "enabled"
  }
  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_size = 50
      volume_type = "gp3"
      encrypted   = true
    }
  }
  tag_specifications {
    resource_type = "instance"
    tags          = { Name = "${var.name}-node" }
  }
}

resource "aws_eks_node_group" "general" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "general"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = var.private_subnet_ids
  instance_types  = ["m6g.large"]         # Graviton: mais barato; nossas imagens são Go, compilam para arm64
  ami_type        = "BOTTLEROCKET_ARM_64" # SO mínimo, imutável, feito para containers
  capacity_type   = "ON_DEMAND"

  scaling_config {
    desired_size = 3
    min_size     = 3
    max_size     = 12
  }

  update_config { max_unavailable = 1 }

  launch_template {
    id      = aws_launch_template.node.id
    version = aws_launch_template.node.latest_version
  }

  depends_on = [aws_iam_role_policy_attachment.node]
}

# ---------- addons ----------
resource "aws_eks_addon" "this" {
  for_each = toset(["vpc-cni", "coredns", "kube-proxy", "eks-pod-identity-agent"])

  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = each.value
  resolve_conflicts_on_update = "OVERWRITE"
  depends_on                  = [aws_eks_node_group.general]
}

# ---------- identidade por pod ----------
# Um pod recebe UMA role IAM, com UMA política mínima. Nada de credenciais em variável de ambiente.

data "aws_iam_policy_document" "pod_identity_trust" {
  statement {
    actions = ["sts:AssumeRole", "sts:TagSession"]
    principals {
      type        = "Service"
      identifiers = ["pods.eks.amazonaws.com"]
    }
  }
}

# External Secrets Operator: lê do Secrets Manager e cria Secrets no cluster. Só os ARNs listados.
resource "aws_iam_role" "external_secrets" {
  name               = "${var.name}-external-secrets"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

resource "aws_iam_role_policy" "external_secrets" {
  role = aws_iam_role.external_secrets.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"]
      Resource = var.secrets_arns
    }]
  })
}

resource "aws_eks_pod_identity_association" "external_secrets" {
  cluster_name    = aws_eks_cluster.this.name
  namespace       = "external-secrets"
  service_account = "external-secrets"
  role_arn        = aws_iam_role.external_secrets.arn
  depends_on      = [aws_eks_addon.this]
}

# Worker de liquidação: só escreve no bucket de liquidação (e nada mais).
resource "aws_iam_role" "settlement" {
  name               = "${var.name}-settlement"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

resource "aws_iam_role_policy" "settlement" {
  role = aws_iam_role.settlement.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["s3:PutObject"]
      Resource = "${var.settlement_bucket}/*"
    }]
  })
}

resource "aws_eks_pod_identity_association" "settlement" {
  cluster_name    = aws_eks_cluster.this.name
  namespace       = "adquirente"
  service_account = "settlement"
  role_arn        = aws_iam_role.settlement.arn
  depends_on      = [aws_eks_addon.this]
}

# Serviços que falam com o MSK via IAM: permissão de conectar e usar tópicos com prefixo do projeto.
resource "aws_iam_role" "kafka_client" {
  name               = "${var.name}-kafka-client"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

resource "aws_iam_role_policy" "kafka_client" {
  role = aws_iam_role.kafka_client.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["kafka-cluster:Connect", "kafka-cluster:DescribeCluster"]
        Resource = "arn:aws:kafka:*:${data.aws_caller_identity.current.account_id}:cluster/${var.name}/*"
      },
      {
        Effect   = "Allow"
        Action   = ["kafka-cluster:*Topic*", "kafka-cluster:WriteData", "kafka-cluster:ReadData"]
        Resource = "arn:aws:kafka:*:${data.aws_caller_identity.current.account_id}:topic/${var.name}/*"
      },
      {
        Effect   = "Allow"
        Action   = ["kafka-cluster:AlterGroup", "kafka-cluster:DescribeGroup"]
        Resource = "arn:aws:kafka:*:${data.aws_caller_identity.current.account_id}:group/${var.name}/*"
      }
    ]
  })
}

resource "aws_eks_pod_identity_association" "kafka_client" {
  for_each        = toset(["api", "scheduler", "notifier", "settlement"])
  cluster_name    = aws_eks_cluster.this.name
  namespace       = "adquirente"
  service_account = each.value
  role_arn        = aws_iam_role.kafka_client.arn
  depends_on      = [aws_eks_addon.this]
}

output "cluster_name" { value = aws_eks_cluster.this.name }
output "cluster_endpoint" { value = aws_eks_cluster.this.endpoint }
output "node_security_group_id" { value = aws_eks_cluster.this.vpc_config[0].cluster_security_group_id }
output "oidc_issuer" { value = aws_eks_cluster.this.identity[0].oidc[0].issuer }

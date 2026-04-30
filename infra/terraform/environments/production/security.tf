# Production Security Configuration
# Security Groups, NACLs, Network Security

# =============================================================================
# Data Sources
# =============================================================================

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# =============================================================================
# Security Groups
# =============================================================================

# EKS Cluster Security Group
resource "aws_security_group" "eks_cluster" {
  name        = "opus-casino-production-eks-cluster-sg"
  description = "Security group for EKS cluster control plane"
  vpc_id      = module.vpc.vpc_id

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    Name        = "eks-cluster-sg"
  }
}

resource "aws_security_group_rule" "eks_cluster_ingress" {
  description       = "Allow HTTPS from VPC"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = ["10.0.0.0/8"]
  security_group_id = aws_security_group.eks_cluster.id
  type              = "ingress"
}

resource "aws_security_group_rule" "eks_cluster_egress" {
  description       = "Allow all outbound"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.eks_cluster.id
  type              = "egress"
}

# EKS Nodes Security Group
resource "aws_security_group" "eks_nodes" {
  name        = "opus-casino-production-eks-nodes-sg"
  description = "Security group for EKS worker nodes"
  vpc_id      = module.vpc.vpc_id

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    Name        = "eks-nodes-sg"
  }
}

resource "aws_security_group_rule" "eks_nodes_ingress_internal" {
  description       = "Allow internal communication"
  from_port         = 0
  to_port           = 65535
  protocol          = "tcp"
  security_group_id = aws_security_group.eks_nodes.id
  type              = "ingress"
  self              = true
}

resource "aws_security_group_rule" "eks_nodes_ingress_cluster" {
  description       = "Allow EKS cluster communication"
  from_port         = 1025
  to_port           = 65535
  protocol          = "tcp"
  source_security_group_id = aws_security_group.eks_cluster.id
  security_group_id = aws_security_group.eks_nodes.id
  type              = "ingress"
}

resource "aws_security_group_rule" "eks_nodes_egress" {
  description       = "Allow all outbound"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.eks_nodes.id
  type              = "egress"
}

# Application Security Group (for services)
resource "aws_security_group" "application" {
  name        = "opus-casino-production-application-sg"
  description = "Security group for application services"
  vpc_id      = module.vpc.vpc_id

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    Name        = "application-sg"
  }
}

resource "aws_security_group_rule" "application_ingress_http" {
  description       = "Allow HTTP from ALB"
  from_port         = 80
  to_port           = 80
  protocol          = "tcp"
  source_security_group_id = aws_security_group.alb.id
  security_group_id = aws_security_group.application.id
  type              = "ingress"
}

resource "aws_security_group_rule" "application_ingress_https" {
  description       = "Allow HTTPS from ALB"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  source_security_group_id = aws_security_group.alb.id
  security_group_id = aws_security_group.application.id
  type              = "ingress"
}

resource "aws_security_group_rule" "application_ingress_internal" {
  description       = "Allow internal service communication"
  from_port         = 8080
  to_port           = 9090
  protocol          = "tcp"
  security_group_id = aws_security_group.application.id
  type              = "ingress"
  self              = true
}

resource "aws_security_group_rule" "application_egress" {
  description       = "Allow all outbound"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.application.id
  type              = "egress"
}

# ALB Security Group
resource "aws_security_group" "alb" {
  name        = "opus-casino-production-alb-sg"
  description = "Security group for Application Load Balancer"
  vpc_id      = module.vpc.vpc_id

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    Name        = "alb-sg"
  }
}

resource "aws_security_group_rule" "alb_ingress_http" {
  description       = "Allow HTTP from CloudFlare"
  from_port         = 80
  to_port           = 80
  protocol          = "tcp"
  cidr_blocks       = ["173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22", "141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13", "104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22"]  # CloudFlare IPs
  security_group_id = aws_security_group.alb.id
  type              = "ingress"
}

resource "aws_security_group_rule" "alb_ingress_https" {
  description       = "Allow HTTPS from CloudFlare"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = ["173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22", "141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13", "104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22"]  # CloudFlare IPs
  security_group_id = aws_security_group.alb.id
  type              = "ingress"
}

resource "aws_security_group_rule" "alb_egress" {
  description       = "Allow outbound to application"
  from_port         = 80
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = ["10.0.0.0/8"]
  security_group_id = aws_security_group.alb.id
  type              = "egress"
}

# Bastion Host Security Group
resource "aws_security_group" "bastion" {
  name        = "opus-casino-production-bastion-sg"
  description = "Security group for bastion host"
  vpc_id      = module.vpc.vpc_id

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    Name        = "bastion-sg"
  }
}

resource "aws_security_group_rule" "bastion_ingress_ssh" {
  description       = "Allow SSH from office IPs"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = var.office_cidr_blocks
  security_group_id = aws_security_group.bastion.id
  type              = "ingress"
}

resource "aws_security_group_rule" "bastion_egress" {
  description       = "Allow outbound to private subnets"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = module.vpc.private_subnet_cidrs
  security_group_id = aws_security_group.bastion.id
  type              = "egress"
}

# =============================================================================
# Network ACLs
# =============================================================================

# Public Subnets NACL
resource "aws_network_acl" "public" {
  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.public_subnet_ids

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    Name        = "public-nacl"
  }
}

resource "aws_network_acl_rule" "public_inbound_http" {
  network_acl_id = aws_network_acl.public.id
  rule_number    = 100
  egress         = false
  protocol       = "tcp"
  rule_action    = "allow"
  cidr_block     = "0.0.0.0/0"
  from_port      = 80
  to_port        = 80
}

resource "aws_network_acl_rule" "public_inbound_https" {
  network_acl_id = aws_network_acl.public.id
  rule_number    = 110
  egress         = false
  protocol       = "tcp"
  rule_action    = "allow"
  cidr_block     = "0.0.0.0/0"
  from_port      = 443
  to_port        = 443
}

resource "aws_network_acl_rule" "public_inbound_ephemeral" {
  network_acl_id = aws_network_acl.public.id
  rule_number    = 120
  egress         = false
  protocol       = "tcp"
  rule_action    = "allow"
  cidr_block     = "0.0.0.0/0"
  from_port      = 1024
  to_port        = 65535
}

resource "aws_network_acl_rule" "public_outbound_all" {
  network_acl_id = aws_network_acl.public.id
  rule_number    = 100
  egress         = true
  protocol       = "-1"
  rule_action    = "allow"
  cidr_block     = "0.0.0.0/0"
  from_port      = 0
  to_port        = 0
}

# Private Subnets NACL
resource "aws_network_acl" "private" {
  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnet_ids

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    Name        = "private-nacl"
  }
}

resource "aws_network_acl_rule" "private_inbound_internal" {
  network_acl_id = aws_network_acl.private.id
  rule_number    = 100
  egress         = false
  protocol       = "tcp"
  rule_action    = "allow"
  cidr_block     = "10.0.0.0/8"
  from_port      = 0
  to_port        = 65535
}

resource "aws_network_acl_rule" "private_inbound_ephemeral" {
  network_acl_id = aws_network_acl.private.id
  rule_number    = 110
  egress         = false
  protocol       = "tcp"
  rule_action    = "allow"
  cidr_block     = "0.0.0.0/0"
  from_port      = 1024
  to_port        = 65535
}

resource "aws_network_acl_rule" "private_outbound_all" {
  network_acl_id = aws_network_acl.private.id
  rule_number    = 100
  egress         = true
  protocol       = "-1"
  rule_action    = "allow"
  cidr_block     = "0.0.0.0/0"
  from_port      = 0
  to_port        = 0
}

# Isolated Subnets NACL (Database tier)
resource "aws_network_acl" "isolated" {
  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.isolated_subnet_ids

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    Name        = "isolated-nacl"
  }
}

resource "aws_network_acl_rule" "isolated_inbound_db" {
  network_acl_id = aws_network_acl.isolated.id
  rule_number    = 100
  egress         = false
  protocol       = "tcp"
  rule_action    = "allow"
  cidr_block     = "10.0.0.0/8"
  from_port      = 5432  # PostgreSQL
  to_port        = 5432
}

resource "aws_network_acl_rule" "isolated_inbound_redis" {
  network_acl_id = aws_network_acl.isolated.id
  rule_number    = 110
  egress         = false
  protocol       = "tcp"
  rule_action    = "allow"
  cidr_block     = "10.0.0.0/8"
  from_port      = 6379  # Redis
  to_port        = 6379
}

resource "aws_network_acl_rule" "isolated_outbound_vpc" {
  network_acl_id = aws_network_acl.isolated.id
  rule_number    = 100
  egress         = true
  protocol       = "-1"
  rule_action    = "allow"
  cidr_block     = "10.0.0.0/8"
  from_port      = 0
  to_port        = 0
}

# =============================================================================
# VPC Flow Logs
# =============================================================================

resource "aws_cloudwatch_log_group" "flow_logs" {
  name              = "/aws/vpc/opus-casino-production-flow-logs"
  retention_in_days = 90

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_iam_role" "flow_logs" {
  name = "opus-casino-production-flow-logs-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "vpc-flow-logs.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_iam_role_policy" "flow_logs" {
  name = "opus-casino-production-flow-logs-policy"
  role = aws_iam_role.flow_logs.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogGroups",
          "logs:DescribeLogStreams"
        ]
        Effect   = "Allow"
        Resource = "*"
      }
    ]
  })
}

resource "aws_flow_log" "main" {
  iam_role_arn    = aws_iam_role.flow_logs.arn
  log_destination = aws_cloudwatch_log_group.flow_logs.arn
  traffic_type    = "ALL"
  vpc_id          = module.vpc.vpc_id

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# =============================================================================
# AWS WAF for ALB
# =============================================================================

resource "aws_wafv2_web_acl" "main" {
  name        = "opus-casino-production-waf"
  description = "WAF for production ALB"
  scope       = "REGIONAL"

  default_action {
    allow {}
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "opus-casino-production-waf"
    sampled_requests_enabled   = true
  }

  # AWS Managed Rules - OWASP Top 10
  rule {
    name     = "AWSManagedRulesCommonRuleSet"
    priority = 1

    override_action {
      none {}
    }

    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesCommonRuleSet"
        vendor_name = "AWS"

        rule_action_override {
          name = "SizeRestrictions_BODY"
          action_to_use {
            allow {}
          }
        }
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "AWSManagedRulesCommonRuleSet"
      sampled_requests_enabled   = true
    }
  }

  # AWS Managed Rules - Known Bad Inputs
  rule {
    name     = "AWSManagedRulesKnownBadInputsRuleSet"
    priority = 2

    override_action {
      none {}
    }

    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesKnownBadInputsRuleSet"
        vendor_name = "AWS"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "AWSManagedRulesKnownBadInputsRuleSet"
      sampled_requests_enabled   = true
    }
  }

  # Rate-based rule for DDoS protection
  rule {
    name     = "RateLimitRule"
    priority = 3

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
      metric_name                = "RateLimitRule"
      sampled_requests_enabled   = true
    }
  }
}

resource "aws_wafv2_web_acl_association" "alb" {
  resource_arn = aws_lb.main.arn
  web_acl_arn  = aws_wafv2_web_acl.main.arn
}

# =============================================================================
# Outputs
# =============================================================================

output "eks_cluster_security_group_id" {
  description = "EKS cluster security group ID"
  value       = aws_security_group.eks_cluster.id
}

output "eks_nodes_security_group_id" {
  description = "EKS nodes security group ID"
  value       = aws_security_group.eks_nodes.id
}

output "application_security_group_id" {
  description = "Application security group ID"
  value       = aws_security_group.application.id
}

output "alb_security_group_id" {
  description = "ALB security group ID"
  value       = aws_security_group.alb.id
}

output "bastion_security_group_id" {
  description = "Bastion security group ID"
  value       = aws_security_group.bastion.id
}

output "waf_web_acl_arn" {
  description = "WAF Web ACL ARN"
  value       = aws_wafv2_web_acl.main.arn
}

output "flow_logs_log_group_arn" {
  description = "VPC Flow Logs CloudWatch log group ARN"
  value       = aws_cloudwatch_log_group.flow_logs.arn
}

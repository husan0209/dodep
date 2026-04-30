variable "environment" {
  description = "Environment name"
  type        = string
}

variable "enable_eks_roles" {
  description = "Create EKS related IAM roles"
  type        = bool
  default     = true
}

variable "enable_rds_roles" {
  description = "Create RDS related IAM roles"
  type        = bool
  default     = true
}

variable "enable_lambda_roles" {
  description = "Create Lambda related IAM roles"
  type        = bool
  default     = false
}

variable "oidc_provider_url" {
  description = "OIDC provider URL for EKS"
  type        = string
  default     = ""
}

variable "tags" {
  description = "Additional tags"
  type        = map(string)
  default     = {}
}

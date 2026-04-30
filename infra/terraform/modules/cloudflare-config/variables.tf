# Variables for CloudFlare Module

variable "account_id" {
  description = "CloudFlare Account ID"
  type        = string
}

variable "domain" {
  description = "Domain name"
  type        = string
}

variable "plan" {
  description = "CloudFlare plan (free, pro, business, enterprise)"
  type        = string
  default     = "business"
}

# API Configuration
variable "api_subdomain" {
  description = "API subdomain"
  type        = string
  default     = "api"
}

# DNS Records
variable "dns_records" {
  description = "DNS records to create"
  type = map(object({
    name    = string
    type    = string
    value   = string
    ttl     = optional(number, 1)
    proxied = optional(bool, true)
    comment = optional(string, null)
    data    = optional(any, null)
  }))
  default = {}
}

# SSL/TLS
variable "custom_ssl_certificate" {
  description = "Custom SSL certificate"
  type        = string
  default     = null
  sensitive   = true
}

variable "custom_ssl_private_key" {
  description = "Custom SSL private key"
  type        = string
  default     = null
  sensitive   = true
}

variable "enable_origin_ca" {
  description = "Enable Origin CA certificate"
  type        = bool
  default     = true
}

variable "origin_hostnames" {
  description = "Hostnames for Origin CA certificate"
  type        = list(string)
  default     = []
}

# Blocked Countries (ISO codes)
variable "blocked_countries" {
  description = "List of blocked country codes (ISO 3166-1 alpha-2)"
  type        = list(string)
  default     = []
}

# Trusted ASNs
variable "trusted_asns" {
  description = "List of trusted ASN numbers"
  type        = list(number)
  default     = []
}

# Rate Limits
variable "rate_limits" {
  description = "Rate limiting configuration"
  type = object({
    login_per_minute       = number
    registration_per_hour  = number
    api_per_minute         = number
    bet_per_10_seconds     = number
    withdrawal_per_hour    = number
  })
  default = {
    login_per_minute      = 10
    registration_per_hour = 5
    api_per_minute        = 100
    bet_per_10_seconds    = 5
    withdrawal_per_hour   = 10
  }
}

# Bot Fight Mode
variable "enable_bot_fight_mode" {
  description = "Enable Bot Fight Mode"
  type        = bool
  default     = true
}

# WWW Redirect
variable "enable_www_redirect" {
  description = "Enable www to non-www redirect"
  type        = bool
  default     = true
}

# Load Balancer
variable "enable_load_balancer" {
  description = "Enable CloudFlare Load Balancer"
  type        = bool
  default     = false
}

variable "load_balancer_name" {
  description = "Load balancer name"
  type        = string
  default     = "main-lb"
}

variable "primary_origin" {
  description = "Primary origin address"
  type        = string
}

variable "secondary_origin" {
  description = "Secondary origin address"
  type        = string
  default     = null
}

variable "fallback_origin" {
  description = "Fallback origin address"
  type        = string
}

variable "api_origins" {
  description = "API origin addresses"
  type        = list(string)
  default     = []
}

variable "health_check_regions" {
  description = "Regions for health checks"
  type        = list(string)
  default     = ["WNAM", "ENAM", "WEU"]
}

# Logpush
variable "enable_logpush" {
  description = "Enable Logpush to S3"
  type        = bool
  default     = true
}

variable "logpush_bucket" {
  description = "S3 bucket for logs"
  type        = string
  default     = null
}

variable "logpush_region" {
  description = "S3 region for logs"
  type        = string
  default     = "us-east-1"
}

# Tags
variable "tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default     = {}
}

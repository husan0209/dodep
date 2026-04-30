# Production CloudFlare Configuration
# DNS, WAF, Rate Limiting, SSL/TLS

# =============================================================================
# Data Sources
# =============================================================================

data "aws_secretsmanager_secret_version" "cloudflare_api_token" {
  secret_id = "opus-casino/production/cloudflare-api-token"
}

# =============================================================================
# CloudFlare Zone
# =============================================================================

resource "cloudflare_zone" "main" {
  account_id = var.cloudflare_account_id
  name       = var.domain
  plan       = "business"
}

# =============================================================================
# Zone Settings
# =============================================================================

resource "cloudflare_zone_settings_override" "main" {
  zone_id = cloudflare_zone.main.id

  # Security settings
  always_use_https         = "on"
  automatic_https_rewrites = "on"
  tls_1_3                  = "on"
  min_tls_version          = "1.3"

  # Performance
  brotli                   = "on"
  minify {
    css  = "on"
    js   = "on"
    html = "on"
  }
  rocket_loader            = "off"

  # Caching
  development_mode         = "off"
  cache_level              = "aggressive"

  # Protection
  security_level           = "high"
  challenge_ttl            = 2592000  # 30 days
  privacy_pass             = "on"

  # Email obfuscation
  email_obfuscation        = "on"
  server_side_exclude      = "on"
}

# =============================================================================
# DNS Records
# =============================================================================

resource "cloudflare_record" "main" {
  for_each = {
    root = {
      name    = "@"
      type    = "A"
      value   = var.lb_ipv4
      proxied = true
    }
    www = {
      name    = "www"
      type    = "CNAME"
      value   = var.domain
      proxied = true
    }
    api = {
      name    = "api"
      type    = "CNAME"
      value   = "lb.opus-casino.com"
      proxied = true
    }
    admin = {
      name    = "admin"
      type    = "A"
      value   = var.admin_ipv4
      proxied = true
    }
    ws = {
      name    = "ws"
      type    = "CNAME"
      value   = "ws-gateway.opus-casino.com"
      proxied = false  # WebSocket не через proxy
    }
  }

  zone_id = cloudflare_zone.main.id
  name    = each.value.name
  type    = each.value.type
  value   = each.value.value
  proxied = each.value.proxied
  comment = "Production DNS record"
}

# =============================================================================
# SSL/TLS Origin Certificate
# =============================================================================

resource "cloudflare_origin_ca_certificate" "main" {
  hostnames      = [var.domain, "*.${var.domain}", "api.${var.domain}", "admin.${var.domain}"]
  request_type   = "origin-rsa"
  requested_validity = 365  # 1 year
}

# =============================================================================
# WAF Rules - OWASP Protection
# =============================================================================

resource "cloudflare_ruleset" "owasp" {
  zone_id     = cloudflare_zone.main.id
  name        = "OWASP Managed Rules"
  description = "OWASP Top 10 protection"
  kind        = "zone"
  phase       = "http_request_security"

  # Block restricted countries
  rules {
    action = "block"
    expression = "(ip.geoip.country in {${join(" ", formatlist("\"%s\"", var.blocked_countries))}})"
    description = "Block restricted countries for gambling compliance"
    enabled = true
  }

  # SQL Injection
  rules {
    action = "block"
    expression = "(http.request.uri.query contains \"union\" and http.request.uri.query contains \"select\") or (http.request.uri.query contains \"insert\" and http.request.uri.query contains \"into\") or (http.request.uri.query contains \"drop\" and http.request.uri.query contains \"table\")"
    description = "Block SQL injection attempts"
    enabled = true
  }

  # XSS Protection
  rules {
    action = "block"
    expression = "(http.request.uri.query contains \"<script\") or (http.request.uri.query contains \"javascript:\") or (http.request.uri.query contains \"onerror=\")"
    description = "Block XSS attempts"
    enabled = true
  }

  # Path Traversal
  rules {
    action = "block"
    expression = "(http.request.uri.path contains \"../\") or (http.request.uri.path contains \"..\\\\\") or (http.request.uri.path contains \"%2e%2e\")"
    description = "Block path traversal attempts"
    enabled = true
  }
}

# =============================================================================
# WAF Rules - Gambling Protection
# =============================================================================

resource "cloudflare_ruleset" "gambling_protection" {
  zone_id     = cloudflare_zone.main.id
  name        = "Gambling Protection"
  description = "Custom rules for gambling platform"
  kind        = "zone"
  phase       = "http_request_security"

  # Block known bot user agents
  rules {
    action = "block"
    expression = "(http.user_agent contains \"python-requests\") or (http.user_agent contains \"curl\") or (http.user_agent contains \"wget\") or (http.user_agent contains \"scrapy\")"
    description = "Block suspicious user agents"
    enabled = true
  }

  # Challenge on auth endpoints
  rules {
    action = "challenge"
    expression = "(http.request.uri.path contains \"/api/auth/\") and (ip.geoip.asnum not in {${join(" ", var.trusted_asns)}})"
    description = "Challenge unknown IPs on auth endpoints"
    enabled = true
  }

  # Block requests without proper headers
  rules {
    action = "block"
    expression = "(not http.request.headers[\"accept\"][0] or not http.request.headers[\"accept-language\"][0])"
    description = "Block requests without standard browser headers"
    enabled = true
  }

  # Protect admin panel
  rules {
    action = "block"
    expression = "(http.request.uri.path contains \"/admin/\") and (ip.geoip.country ne \"US\")"
    description = "Block admin access from outside US"
    enabled = true
  }
}

# =============================================================================
# Rate Limiting Rules
# =============================================================================

resource "cloudflare_ruleset" "rate_limiting" {
  zone_id     = cloudflare_zone.main.id
  name        = "Rate Limiting"
  description = "API rate limiting rules"
  kind        = "zone"
  phase       = "http_ratelimit"

  # Login rate limit
  rules {
    action = "block"
    action_parameters {
      response {
        status_code  = 429
        content_type = "application/json"
        content      = "{\"error\": \"Too many login attempts\"}"
      }
    }
    expression = "(http.request.uri.path contains \"/api/auth/login\")"
    description = "Rate limit login endpoint"
    enabled = true

    ratelimit {
      characteristics = ["ip.src", "http.request.uri.path"]
      period          = 60
      requests_per_period = 10
      mitigation_timeout = 300
    }
  }

  # Registration rate limit
  rules {
    action = "block"
    action_parameters {
      response {
        status_code  = 429
        content_type = "application/json"
        content      = "{\"error\": \"Too many registration attempts\"}"
      }
    }
    expression = "(http.request.uri.path contains \"/api/auth/register\")"
    description = "Rate limit registration endpoint"
    enabled = true

    ratelimit {
      characteristics = ["ip.src"]
      period          = 3600
      requests_per_period = 5
      mitigation_timeout = 3600
    }
  }

  # API general rate limit (per user)
  rules {
    action = "block"
    action_parameters {
      response {
        status_code  = 429
        content_type = "application/json"
        content      = "{\"error\": \"Rate limit exceeded\"}"
      }
    }
    expression = "(http.request.uri.path contains \"/api/\") and (http.request.headers[\"authorization\"][0] ne \"\")"
    description = "Rate limit authenticated API requests"
    enabled = true

    ratelimit {
      characteristics = ["http.request.headers[\"cf-connecting-ip\"]", "http.request.headers[\"authorization\"]"]
      period          = 60
      requests_per_period = 100
      mitigation_timeout = 60
    }
  }

  # Bet placement rate limit
  rules {
    action = "block"
    action_parameters {
      response {
        status_code  = 429
        content_type = "application/json"
        content      = "{\"error\": \"Too many bet attempts\"}"
      }
    }
    expression = "(http.request.uri.path contains \"/api/bets/place\")"
    description = "Rate limit bet placement"
    enabled = true

    ratelimit {
      characteristics = ["http.request.headers[\"cf-connecting-ip\"]", "http.request.headers[\"authorization\"]"]
      period          = 10
      requests_per_period = 5
      mitigation_timeout = 30
    }
  }

  # Withdrawal rate limit
  rules {
    action = "block"
    action_parameters {
      response {
        status_code  = 429
        content_type = "application/json"
        content      = "{\"error\": \"Too many withdrawal attempts\"}"
      }
    }
    expression = "(http.request.uri.path contains \"/api/wallet/withdraw\")"
    description = "Rate limit withdrawal requests"
    enabled = true

    ratelimit {
      characteristics = ["http.request.headers[\"cf-connecting-ip\"]", "http.request.headers[\"authorization\"]"]
      period          = 3600
      requests_per_period = 10
      mitigation_timeout = 3600
    }
  }

  # Password reset rate limit
  rules {
    action = "block"
    action_parameters {
      response {
        status_code  = 429
        content_type = "application/json"
        content      = "{\"error\": \"Too many password reset attempts\"}"
      }
    }
    expression = "(http.request.uri.path contains \"/api/auth/reset-password\")"
    description = "Rate limit password reset"
    enabled = true

    ratelimit {
      characteristics = ["ip.src"]
      period          = 3600
      requests_per_period = 3
      mitigation_timeout = 3600
    }
  }
}

# =============================================================================
# Bot Fight Mode
# =============================================================================

resource "cloudflare_bot_management" "main" {
  zone_id = cloudflare_zone.main.id

  fight_mode               = true
  enable_js                = true
  optimize_wordpress       = false
  using_default_adaptive   = true
}

# =============================================================================
# Page Rules
# =============================================================================

# API - no cache
resource "cloudflare_page_rule" "api_no_cache" {
  zone_id = cloudflare_zone.main.id
  target  = "api.${var.domain}/*"

  actions {
    cache_level = "bypass"
    disable_apps = true
  }
}

# Static assets - cache everything
resource "cloudflare_page_rule" "static_cache" {
  zone_id = cloudflare_zone.main.id
  target  = "${var.domain}/assets/*"

  actions {
    cache_level = "cache_everything"
    edge_cache_ttl = 604800  # 7 days
    browser_cache_ttl = 86400  # 1 day
  }
}

# WWW redirect
resource "cloudflare_page_rule" "www_redirect" {
  zone_id = cloudflare_zone.main.id
  target  = "www.${var.domain}/*"

  actions {
    redirect_url = "https://${var.domain}/$1"
    status_code  = 301
  }
}

# =============================================================================
# Load Balancer
# =============================================================================

resource "cloudflare_load_balancer" "main" {
  zone_id          = cloudflare_zone.main.id
  name             = "lb"
  default_pool_ids = [cloudflare_load_balancer_pool.main.id]
  fallback_pool_id = cloudflare_load_balancer_pool.fallback.id
  proxied          = true
  steering_policy  = "geo"

  session_affinity       = "cookie"
  session_affinity_ttl   = 1800
  session_affinity_attributes {
    samesite = "Strict"
    secure   = "Always"
  }
}

resource "cloudflare_load_balancer_pool" "main" {
  account_id               = var.cloudflare_account_id
  name                     = "primary-pool"
  load_balancing_algorithm = "least_outstanding_requests"

  origins {
    name    = "us-east-origin"
    address = var.primary_origin
    enabled = true
    weight  = 1.0
  }

  origins {
    name    = "us-west-origin"
    address = var.secondary_origin
    enabled = true
    weight  = 1.0
  }

  check_regions = ["WNAM", "ENAM", "WEU"]
}

resource "cloudflare_load_balancer_pool" "fallback" {
  account_id               = var.cloudflare_account_id
  name                     = "fallback-pool"
  load_balancing_algorithm = "least_outstanding_requests"

  origins {
    name    = "fallback-origin"
    address = var.fallback_origin
    enabled = true
    weight  = 1.0
  }
}

# =============================================================================
# Logpush
# =============================================================================

resource "cloudflare_logpush_job" "http_logs" {
  zone_id          = cloudflare_zone.main.id
  enabled          = true
  name             = "HTTP Logs to S3"
  logpull_options  = "fields=RayID,ClientIP,ClientRequestHost,ClientRequestMethod,ClientRequestURI,ClientRequestProtocol,ClientRequestUserAgent,EdgeStartTimestamp,EdgeEndTimestamp,EdgeResponseStatus,EdgeCacheStatus,SecurityLevel,SmartType,OriginResponseStatus,ClientSSLProtocol,SecurityRuleID,SecurityRuleMessage,SecurityRuleActions,ClientBotScore,ClientVerifiedBot,ClientCountry,ClientRegion,ClientCity&timestamps=iso8601"
  destination_conf = "s3://${var.logpush_bucket}?region=${var.logpush_region}&accountid=${var.cloudflare_account_id}"
  dataset          = "http_requests"
  frequency        = "high"
}

# =============================================================================
# Outputs
# =============================================================================

output "zone_id" {
  description = "CloudFlare Zone ID"
  value       = cloudflare_zone.main.id
}

output "zone_name_servers" {
  description = "CloudFlare name servers"
  value       = cloudflare_zone.main.name_servers
}

output "origin_ca_certificate" {
  description = "Origin CA certificate"
  value       = cloudflare_origin_ca_certificate.main.certificate
  sensitive   = true
}

output "origin_ca_private_key" {
  description = "Origin CA private key"
  value       = cloudflare_origin_ca_certificate.main.private_key
  sensitive   = true
}

output "load_balancer_hostname" {
  description = "Load balancer hostname"
  value       = cloudflare_load_balancer.main.hostname
}

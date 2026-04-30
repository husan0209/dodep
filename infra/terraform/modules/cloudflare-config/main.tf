# CloudFlare Module for Opus Casino
# DNS, WAF, Rate Limiting, SSL/TLS, Bot Management

# =============================================================================
# Zone Settings
# =============================================================================

resource "cloudflare_zone" "main" {
  account_id = var.account_id
  name       = var.domain
  plan       = var.plan
}

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
  rocket_loader            = "off"  # Off для dynamic content

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
# SSL/TLS Configuration
# =============================================================================

resource "cloudflare_custom_ssl" "main" {
  count = var.custom_ssl_certificate != null ? 1 : 0

  zone_id = cloudflare_zone.main.id

  custom_ssl_options {
    certificate   = var.custom_ssl_certificate
    private_key   = var.custom_ssl_private_key
    type          = "legacy_custom"
  }
}

# SSL/TLS Origin Certificate (для backend)
resource "cloudflare_origin_ca_certificate" "main" {
  count = var.enable_origin_ca ? 1 : 0

  hostnames = var.origin_hostnames
  request_type = "origin-rsa"
  requested_validity = 365  # 1 year
}

# =============================================================================
# DNS Records
# =============================================================================

resource "cloudflare_record" "main" {
  for_each = var.dns_records

  zone_id = cloudflare_zone.main.id
  name    = each.value.name
  type    = each.value.type
  value   = each.value.value
  ttl     = lookup(each.value, "ttl", 1)  # Auto TTL
  proxied = lookup(each.value, "proxied", true)
  comment = lookup(each.value, "comment", null)

  dynamic "data" {
    for_each = lookup(each.value, "data", null) != null ? [each.value.data] : []
    content {
      http_port   = lookup(data.value, "http_port", null)
      http_path   = lookup(data.value, "http_path", null)
      http_protocol = lookup(data.value, "http_protocol", null)
    }
  }
}

# =============================================================================
# WAF Rules
# =============================================================================

# OWASP Managed Rules
resource "cloudflare_ruleset" "owasp" {
  zone_id     = cloudflare_zone.main.id
  name        = "OWASP Managed Rules"
  description = "OWASP Top 10 protection"
  kind        = "zone"
  phase       = "http_request_security"

  rules {
    action = "block"
    expression = "(ip.geoip.country in {${join(" ", formatlist("\"%s\"", var.blocked_countries))}})"
    description = "Block restricted countries"
    enabled = true
  }

  # SQL Injection
  rules {
    action = "block"
    expression = "(http.request.uri.query contains \"union\" and http.request.uri.query contains \"select\") or (http.request.uri.query contains \"insert\" and http.request.uri.query contains \"into\")"
    description = "Block SQL injection attempts"
    enabled = true
  }

  # XSS Protection
  rules {
    action = "block"
    expression = "(http.request.uri.query contains \"<script\") or (http.request.uri.query contains \"javascript:\")"
    description = "Block XSS attempts"
    enabled = true
  }

  # Path Traversal
  rules {
    action = "block"
    expression = "(http.request.uri.path contains \"../\") or (http.request.uri.path contains \"..\\\\\")"
    description = "Block path traversal attempts"
    enabled = true
  }
}

# Custom WAF Rules for Gambling
resource "cloudflare_ruleset" "gambling_protection" {
  zone_id     = cloudflare_zone.main.id
  name        = "Gambling Protection"
  description = "Custom rules for gambling platform"
  kind        = "zone"
  phase       = "http_request_security"

  # Block known bot user agents
  rules {
    action = "block"
    expression = "(http.user_agent contains \"python-requests\") or (http.user_agent contains \"curl\") or (http.user_agent contains \"wget\")"
    description = "Block suspicious user agents"
    enabled = true
  }

  # Protect sensitive endpoints
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
    description = "Block requests without standard headers"
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
      requests_per_period = var.rate_limits.login_per_minute
      mitigation_timeout = 300  # 5 minutes block
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
      requests_per_period = var.rate_limits.registration_per_hour
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
      requests_per_period = var.rate_limits.api_per_minute
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
      requests_per_period = var.rate_limits.bet_per_10_seconds
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
      requests_per_period = var.rate_limits.withdrawal_per_hour
      mitigation_timeout = 3600
    }
  }
}

# =============================================================================
# Bot Fight Mode
# =============================================================================

resource "cloudflare_bot_management" "main" {
  zone_id = cloudflare_zone.main.id

  fight_mode               = var.enable_bot_fight_mode
  enable_js                = true
  optimize_wordpress       = false
  using_default_adaptive   = true
}

# =============================================================================
# Page Rules (Caching, Redirects)
# =============================================================================

resource "cloudflare_page_rule" "api_no_cache" {
  zone_id = cloudflare_zone.main.id
  target  = "${var.api_subdomain}.${var.domain}/*"

  actions {
    cache_level = "bypass"
    disable_apps = true
  }
}

resource "cloudflare_page_rule" "static_cache" {
  zone_id = cloudflare_zone.main.id
  target  = "${var.domain}/assets/*"

  actions {
    cache_level = "cache_everything"
    edge_cache_ttl = 604800  # 7 days
    browser_cache_ttl = 86400  # 1 day
  }
}

resource "cloudflare_page_rule" "www_redirect" {
  count = var.enable_www_redirect ? 1 : 0

  zone_id = cloudflare_zone.main.id
  target  = "www.${var.domain}/*"

  actions {
    redirect_url = "https://${var.domain}/$1"
    status_code  = 301
  }
}

# =============================================================================
# Load Balancer (optional)
# =============================================================================

resource "cloudflare_load_balancer" "main" {
  count = var.enable_load_balancer ? 1 : 0

  zone_id                  = cloudflare_zone.main.id
  name                     = var.load_balancer_name
  default_pool_ids         = [cloudflare_load_balancer_pool.main[0].id]
  fallback_pool_id         = cloudflare_load_balancer_pool.fallback[0].id
  proxied                  = true
  steering_policy          = "geo"
  persist_tls_session_id   = true
  session_affinity         = "cookie"
  session_affinity_ttl     = 1800  # 30 minutes
  session_affinity_attributes {
    samesite = "Strict"
    secure   = "Always"
  }

  rules {
    name      = "api-routing"
    condition = "(http.request.uri.path contains \"/api/\")"
    priority  = 1

    turn_off_health_checks = false

    overrides {
      default_pools = [cloudflare_load_balancer_pool.api[0].id]
    }
  }
}

resource "cloudflare_load_balancer_pool" "main" {
  count = var.enable_load_balancer ? 1 : 0

  account_id   = var.account_id
  name         = "primary-pool"
  load_balancing_algorithm = "least_outstanding_requests"

  origins {
    name    = "primary-origin"
    address = var.primary_origin
    enabled = true
    weight  = 1.0
  }

  origins {
    name    = "secondary-origin"
    address = var.secondary_origin
    enabled = true
    weight  = 1.0
  }

  check_regions = var.health_check_regions
}

resource "cloudflare_load_balancer_pool" "fallback" {
  count = var.enable_load_balancer ? 1 : 0

  account_id   = var.account_id
  name         = "fallback-pool"
  load_balancing_algorithm = "least_outstanding_requests"

  origins {
    name    = "fallback-origin"
    address = var.fallback_origin
    enabled = true
    weight  = 1.0
  }
}

resource "cloudflare_load_balancer_pool" "api" {
  count = var.enable_load_balancer ? 1 : 0

  account_id   = var.account_id
  name         = "api-pool"
  load_balancing_algorithm = "least_outstanding_requests"

  origins {
    name    = "api-origin-1"
    address = var.api_origins[0]
    enabled = true
    weight  = 1.0
  }

  origins {
    name    = "api-origin-2"
    address = var.api_origins[1]
    enabled = true
    weight  = 1.0
  }

  check_regions = var.health_check_regions
}

# =============================================================================
# Health Checks
# =============================================================================

resource "cloudflare_load_balancer_monitor" "http" {
  count = var.enable_load_balancer ? 1 : 0

  account_id       = var.account_id
  type             = "http"
  description      = "HTTP health check"
  interval         = 30
  timeout          = 5
  retries          = 3
  path             = "/health"
  expected_body    = "healthy"
  expected_codes   = "200"
  follow_redirects = false
  allow_insecure   = false
}

# =============================================================================
# Analytics
# =============================================================================

resource "cloudflare_logpush_job" "http_logs" {
  count = var.enable_logpush ? 1 : 0

  zone_id          = cloudflare_zone.main.id
  enabled          = true
  name             = "HTTP Logs to S3"
  logpull_options  = "fields=RayID,ClientIP,ClientRequestHost,ClientRequestMethod,ClientRequestURI,ClientRequestProtocol,ClientRequestUserAgent,EdgeStartTimestamp,EdgeEndTimestamp,EdgeResponseStatus,EdgeCacheStatus,SecurityLevel,SmartType,OriginResponseStatus,OriginSSLProtocol,ClientSSLProtocol,WorkerStatus,WorkerCPUTime,WorkerSubrequest,WorkerSubrequestCount,WorkerExecutionMode,WorkerWallTimeUs,ParentRayID,RequestHeaders,ResponseHeaders,SecurityRuleID,SecurityRuleMessage,SecurityRuleActions,SecurityRuleMatched,SecurityRuleScore,ClientTrustScore,ClientThreatScore,ClientBotScore,ClientBotTags,ClientVerifiedBot,ClientVerifiedBotCategory,ClientVerifiedBotProducer,ClientAsOrganization,ClientAsDomain,ClientIsp,ClientIspType,ClientConnectionType,ClientDeviceType,ClientCountry,ClientRegion,ClientCity,ClientMetroCode,ClientPostalCode,ClientLatitude,ClientLongitude,ClientTimeZone,ClientAsNumber,ClientAsName,ClientDomain,ClientIspName,ClientIspType,ClientConnectionType,ClientDeviceType,ClientCountry,ClientRegion,ClientCity,ClientMetroCode,ClientPostalCode,ClientLatitude,ClientLongitude,ClientTimeZone,ClientAsNumber,ClientAsName&timestamps=iso8601"
  destination_conf = "s3://${var.logpush_bucket}?region=${var.logpush_region}&accountid=${var.account_id}"
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
  description = "Name servers for the zone"
  value       = cloudflare_zone.main.name_servers
}

output "origin_ca_certificate" {
  description = "Origin CA certificate"
  value       = var.enable_origin_ca ? cloudflare_origin_ca_certificate.main[0].certificate : null
  sensitive   = true
}

output "origin_ca_private_key" {
  description = "Origin CA private key"
  value       = var.enable_origin_ca ? cloudflare_origin_ca_certificate.main[0].private_key : null
  sensitive   = true
}

output "load_balancer_hostname" {
  description = "Load balancer hostname"
  value       = var.enable_load_balancer ? cloudflare_load_balancer.main[0].hostname : null
}

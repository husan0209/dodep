# Outputs for CloudFlare Module

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

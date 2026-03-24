# Variables for Opus Casino infrastructure

variable "project_id" {
  description = "GCP Project ID"
  type        = string
  default     = "opus-casino-prod"
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "us-central1"
}

variable "cluster_name" {
  description = "GKE Cluster name"
  type        = string
  default     = "opus-casino-cluster"
}

variable "node_count" {
  description = "Number of nodes per zone"
  type        = number
  default     = 5
}

variable "machine_type" {
  description = "GCE machine type"
  type        = string
  default     = "n2-standard-4"
}

variable "min_nodes" {
  description = "Minimum nodes for autoscaling"
  type        = number
  default     = 3
}

variable "max_nodes" {
  description = "Maximum nodes for autoscaling"
  type        = number
  default     = 20
}

variable "disk_size_gb" {
  description = "Boot disk size in GB"
  type        = number
  default     = 100
}

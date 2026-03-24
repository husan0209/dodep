variable "environment" {
  description = "Environment name"
  type        = string
}

variable "keys" {
  description = "Map of KMS keys to create"
  type = map(object({
    description              = optional(string, "")
    deletion_window_in_days  = optional(number, 30)
    enable_key_rotation      = optional(bool, true)
    multi_region             = optional(bool, false)
    customers                = optional(list(string), [])
    services                 = optional(list(string), [])
  }))
  default = {}
}

variable "tags" {
  description = "Additional tags"
  type        = map(string)
  default     = {}
}

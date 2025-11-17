variable "domain_name" {
  description = "Custom domain name for API Gateway"
  type        = string
}

variable "api_id" {
  description = "ID of the API Gateway"
  type        = string
}

variable "api_stage_name" {
  description = "Name of the API Gateway stage"
  type        = string
}

variable "route53_zone_id" {
  description = "Route53 hosted zone ID"
  type        = string
}

variable "tags" {
  description = "Tags to apply to custom domain resources"
  type        = map(string)
  default     = {}
}

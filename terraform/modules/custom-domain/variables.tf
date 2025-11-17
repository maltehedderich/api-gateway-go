variable "domain_name" {
  description = "Custom domain name"
  type        = string
}

variable "certificate_arn" {
  description = "ARN of ACM certificate"
  type        = string
}

variable "api_gateway_id" {
  description = "API Gateway ID"
  type        = string
}

variable "api_gateway_stage_name" {
  description = "API Gateway stage name"
  type        = string
}

variable "api_gateway_stage_invoke_url" {
  description = "API Gateway stage invoke URL"
  type        = string
}

variable "route53_zone_id" {
  description = "Route53 hosted zone ID (leave empty to skip DNS record creation)"
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default     = {}
}

variable "name" {
  description = "Name of the API Gateway"
  type        = string
}

variable "description" {
  description = "Description of the API Gateway"
  type        = string
  default     = ""
}

variable "throttle_burst_limit" {
  description = "Throttle burst limit"
  type        = number
  default     = 100
}

variable "throttle_rate_limit" {
  description = "Throttle rate limit (requests per second)"
  type        = number
  default     = 50
}

variable "enable_jwt_authorizer" {
  description = "Enable JWT authorizer"
  type        = bool
  default     = true
}

variable "jwt_issuer" {
  description = "JWT issuer URL"
  type        = string
  default     = ""
}

variable "jwt_audience" {
  description = "JWT audience list"
  type        = list(string)
  default     = []
}

variable "jwt_identity_source" {
  description = "JWT identity source"
  type        = string
  default     = "$request.header.Authorization"
}

variable "enable_cors" {
  description = "Enable CORS"
  type        = bool
  default     = true
}

variable "cors_allow_origins" {
  description = "CORS allowed origins"
  type        = list(string)
  default     = ["*"]
}

variable "cors_allow_methods" {
  description = "CORS allowed methods"
  type        = list(string)
  default     = ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
}

variable "cors_allow_headers" {
  description = "CORS allowed headers"
  type        = list(string)
  default     = ["Content-Type", "Authorization"]
}

variable "cors_max_age" {
  description = "CORS max age in seconds"
  type        = number
  default     = 86400
}

variable "routes" {
  description = "Map of routes to Lambda integrations"
  type = map(object({
    lambda_invoke_arn  = string
    authorization_type = string
    throttle_settings = optional(object({
      burst_limit = number
      rate_limit  = number
    }))
  }))
}

variable "lambda_permissions" {
  description = "Map of Lambda function names for permissions"
  type = map(object({
    function_name = string
  }))
}

variable "enable_access_logs" {
  description = "Enable access logs"
  type        = bool
  default     = true
}

variable "log_retention_days" {
  description = "CloudWatch log retention in days"
  type        = number
  default     = 7
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default     = {}
}

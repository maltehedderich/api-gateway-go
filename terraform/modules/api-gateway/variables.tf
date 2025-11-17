variable "api_name" {
  description = "Name of the API Gateway"
  type        = string
}

variable "lambda_function_arn" {
  description = "ARN of the Lambda function"
  type        = string
}

variable "lambda_invoke_arn" {
  description = "Invoke ARN of the Lambda function"
  type        = string
}

variable "stage_name" {
  description = "Name of the API Gateway stage"
  type        = string
  default     = "$default"
}

variable "log_retention_days" {
  description = "CloudWatch Logs retention period in days"
  type        = number
  default     = 7
}

# CORS Configuration
variable "cors_allow_origins" {
  description = "CORS allowed origins"
  type        = list(string)
  default     = ["*"]
}

variable "cors_allow_methods" {
  description = "CORS allowed methods"
  type        = list(string)
  default     = ["GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"]
}

variable "cors_allow_headers" {
  description = "CORS allowed headers"
  type        = list(string)
  default     = ["*"]
}

variable "cors_max_age" {
  description = "CORS max age in seconds"
  type        = number
  default     = 86400
}

# Throttling Configuration
variable "throttling_burst_limit" {
  description = "API Gateway throttling burst limit"
  type        = number
  default     = 100
}

variable "throttling_rate_limit" {
  description = "API Gateway throttling rate limit (requests per second)"
  type        = number
  default     = 50
}

variable "integration_timeout_ms" {
  description = "Integration timeout in milliseconds (50-30000)"
  type        = number
  default     = 30000
}

variable "tags" {
  description = "Tags to apply to API Gateway resources"
  type        = map(string)
  default     = {}
}

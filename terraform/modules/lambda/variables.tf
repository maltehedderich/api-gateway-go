variable "function_name" {
  description = "Name of the Lambda function"
  type        = string
}

variable "execution_role_arn" {
  description = "ARN of the Lambda execution role"
  type        = string
}

variable "image_uri" {
  description = "ECR image URI for the Lambda function"
  type        = string
  default     = "public.ecr.aws/lambda/provided:al2023" # Placeholder - must be replaced
}

variable "memory_size" {
  description = "Memory size for Lambda function in MB"
  type        = number
  default     = 512
}

variable "timeout" {
  description = "Timeout for Lambda function in seconds"
  type        = number
  default     = 30
}

variable "environment_variables" {
  description = "Environment variables for Lambda function"
  type        = map(string)
  default     = {}
}

variable "log_retention_days" {
  description = "CloudWatch Logs retention period in days"
  type        = number
  default     = 7
}

variable "reserved_concurrent_executions" {
  description = "Reserved concurrent executions for the Lambda function"
  type        = number
  default     = null
}

variable "tags" {
  description = "Tags to apply to Lambda resources"
  type        = map(string)
  default     = {}
}

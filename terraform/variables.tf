# Root Module Variables
# Configure these values for your deployment

variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (e.g., dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
  default     = "api-gateway"
}

# Lambda Configuration
variable "lambda_image_uri" {
  description = "ECR image URI for Lambda function (e.g., 123456789012.dkr.ecr.us-east-1.amazonaws.com/api-gateway:latest)"
  type        = string
}

variable "lambda_memory_size" {
  description = "Lambda function memory in MB (128-10240)"
  type        = number
  default     = 512
  validation {
    condition     = var.lambda_memory_size >= 128 && var.lambda_memory_size <= 10240
    error_message = "Lambda memory must be between 128 and 10240 MB."
  }
}

variable "lambda_timeout" {
  description = "Lambda function timeout in seconds (max 900)"
  type        = number
  default     = 30
  validation {
    condition     = var.lambda_timeout > 0 && var.lambda_timeout <= 900
    error_message = "Lambda timeout must be between 1 and 900 seconds."
  }
}

variable "lambda_log_retention_days" {
  description = "CloudWatch Logs retention period in days"
  type        = number
  default     = 7
  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653], var.lambda_log_retention_days)
    error_message = "Log retention must be a valid CloudWatch Logs retention period."
  }
}

# DynamoDB Configuration
variable "dynamodb_billing_mode" {
  description = "DynamoDB billing mode (PAY_PER_REQUEST or PROVISIONED)"
  type        = string
  default     = "PAY_PER_REQUEST"
  validation {
    condition     = contains(["PAY_PER_REQUEST", "PROVISIONED"], var.dynamodb_billing_mode)
    error_message = "Billing mode must be PAY_PER_REQUEST or PROVISIONED."
  }
}

variable "dynamodb_read_capacity" {
  description = "DynamoDB read capacity units (only for PROVISIONED billing mode)"
  type        = number
  default     = 5
}

variable "dynamodb_write_capacity" {
  description = "DynamoDB write capacity units (only for PROVISIONED billing mode)"
  type        = number
  default     = 5
}

# Backend Service Configuration
variable "backend_users_url" {
  description = "Backend URL for users service (e.g., https://users-api.example.com)"
  type        = string
  default     = ""
}

variable "backend_orders_url" {
  description = "Backend URL for orders service (e.g., https://orders-api.example.com)"
  type        = string
  default     = ""
}

# Custom Domain (Optional)
variable "enable_custom_domain" {
  description = "Enable custom domain for API Gateway"
  type        = bool
  default     = false
}

variable "domain_name" {
  description = "Custom domain name for API Gateway (e.g., api.example.com)"
  type        = string
  default     = ""
}

variable "route53_zone_id" {
  description = "Route53 hosted zone ID for custom domain"
  type        = string
  default     = ""
}

# Gateway Configuration
variable "log_level" {
  description = "Log level for the gateway (debug, info, warn, error)"
  type        = string
  default     = "info"
  validation {
    condition     = contains(["debug", "info", "warn", "error"], var.log_level)
    error_message = "Log level must be one of: debug, info, warn, error."
  }
}

variable "enable_authorization" {
  description = "Enable JWT authorization"
  type        = bool
  default     = false
}

variable "enable_rate_limiting" {
  description = "Enable rate limiting"
  type        = bool
  default     = true
}

variable "jwt_shared_secret" {
  description = "JWT shared secret for token validation (use AWS Secrets Manager in production)"
  type        = string
  default     = ""
  sensitive   = true
}

# Resource Tagging
variable "tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}

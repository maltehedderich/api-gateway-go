# ========================================
# General Configuration
# ========================================

variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be dev, staging, or prod."
  }
}

variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
  default     = "api-gateway"
}

variable "common_tags" {
  description = "Common tags applied to all resources"
  type        = map(string)
  default = {
    Project     = "api-gateway"
    ManagedBy   = "terraform"
    CostCenter  = "engineering"
  }
}

# ========================================
# API Gateway Configuration
# ========================================

variable "api_gateway_throttle_burst_limit" {
  description = "API Gateway throttle burst limit (max concurrent requests)"
  type        = number
  default     = 100
}

variable "api_gateway_throttle_rate_limit" {
  description = "API Gateway throttle rate limit (requests per second)"
  type        = number
  default     = 50
}

variable "enable_api_gateway_logs" {
  description = "Enable API Gateway access logs"
  type        = bool
  default     = true
}

variable "api_gateway_log_retention_days" {
  description = "CloudWatch log retention in days for API Gateway"
  type        = number
  default     = 7

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653], var.api_gateway_log_retention_days)
    error_message = "Log retention must be a valid CloudWatch Logs retention value."
  }
}

# ========================================
# JWT Authorization Configuration
# ========================================

variable "enable_jwt_authorizer" {
  description = "Enable JWT authorizer for API Gateway"
  type        = bool
  default     = true
}

variable "jwt_audience" {
  description = "JWT audience claim (aud) - list of valid audiences"
  type        = list(string)
  default     = ["api-gateway"]
}

variable "jwt_issuer" {
  description = "JWT issuer claim (iss) - URL of token issuer"
  type        = string
  default     = "https://auth.example.com"
}

variable "jwt_identity_source" {
  description = "JWT token source - where to extract the JWT token from"
  type        = string
  default     = "$request.header.Authorization"
}

# ========================================
# Lambda Configuration
# ========================================

variable "lambda_runtime" {
  description = "Lambda runtime for functions"
  type        = string
  default     = "go1.x"
}

variable "lambda_memory_size" {
  description = "Memory allocation for Lambda functions (MB)"
  type        = number
  default     = 256

  validation {
    condition     = var.lambda_memory_size >= 128 && var.lambda_memory_size <= 10240
    error_message = "Lambda memory must be between 128 MB and 10240 MB."
  }
}

variable "lambda_timeout" {
  description = "Timeout for Lambda functions (seconds)"
  type        = number
  default     = 10

  validation {
    condition     = var.lambda_timeout >= 1 && var.lambda_timeout <= 900
    error_message = "Lambda timeout must be between 1 and 900 seconds."
  }
}

variable "lambda_log_retention_days" {
  description = "CloudWatch log retention in days for Lambda functions"
  type        = number
  default     = 7

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653], var.lambda_log_retention_days)
    error_message = "Log retention must be a valid CloudWatch Logs retention value."
  }
}

variable "lambda_reserved_concurrent_executions" {
  description = "Reserved concurrent executions for Lambda functions (-1 for unreserved)"
  type        = number
  default     = -1
}

variable "lambda_environment_variables" {
  description = "Environment variables for Lambda functions"
  type        = map(string)
  default     = {}
  sensitive   = true
}

# ========================================
# DynamoDB Configuration
# ========================================

variable "enable_dynamodb" {
  description = "Enable DynamoDB table creation"
  type        = bool
  default     = true
}

variable "dynamodb_billing_mode" {
  description = "DynamoDB billing mode (PROVISIONED or PAY_PER_REQUEST)"
  type        = string
  default     = "PAY_PER_REQUEST"

  validation {
    condition     = contains(["PROVISIONED", "PAY_PER_REQUEST"], var.dynamodb_billing_mode)
    error_message = "Billing mode must be PROVISIONED or PAY_PER_REQUEST."
  }
}

variable "dynamodb_point_in_time_recovery" {
  description = "Enable point-in-time recovery for DynamoDB"
  type        = bool
  default     = false # Set to true for production
}

variable "dynamodb_deletion_protection" {
  description = "Enable deletion protection for DynamoDB tables"
  type        = bool
  default     = false # Set to true for production
}

variable "dynamodb_tables" {
  description = "DynamoDB table configurations"
  type = map(object({
    hash_key           = string
    range_key          = optional(string)
    attributes         = list(object({
      name = string
      type = string # S (string), N (number), B (binary)
    }))
    global_secondary_indexes = optional(list(object({
      name            = string
      hash_key        = string
      range_key       = optional(string)
      projection_type = string
    })), [])
    stream_enabled   = optional(bool, false)
    stream_view_type = optional(string, "NEW_AND_OLD_IMAGES")
  }))
  default = {
    # Example table for user data
    users = {
      hash_key = "user_id"
      attributes = [
        { name = "user_id", type = "S" },
        { name = "email", type = "S" }
      ]
      global_secondary_indexes = [
        {
          name            = "email-index"
          hash_key        = "email"
          projection_type = "ALL"
        }
      ]
    }
    # Example table for orders
    orders = {
      hash_key  = "order_id"
      range_key = "created_at"
      attributes = [
        { name = "order_id", type = "S" },
        { name = "created_at", type = "N" },
        { name = "user_id", type = "S" }
      ]
      global_secondary_indexes = [
        {
          name            = "user-orders-index"
          hash_key        = "user_id"
          range_key       = "created_at"
          projection_type = "ALL"
        }
      ]
    }
  }
}

# ========================================
# Custom Domain Configuration (Optional)
# ========================================

variable "enable_custom_domain" {
  description = "Enable custom domain for API Gateway"
  type        = bool
  default     = false
}

variable "domain_name" {
  description = "Custom domain name for API Gateway"
  type        = string
  default     = ""
}

variable "certificate_arn" {
  description = "ARN of ACM certificate for custom domain (must be in us-east-1 for edge-optimized)"
  type        = string
  default     = ""
}

variable "route53_zone_id" {
  description = "Route53 hosted zone ID for custom domain DNS record"
  type        = string
  default     = ""
}

# ========================================
# CORS Configuration
# ========================================

variable "enable_cors" {
  description = "Enable CORS configuration for API Gateway"
  type        = bool
  default     = true
}

variable "cors_allow_origins" {
  description = "CORS allowed origins"
  type        = list(string)
  default     = ["*"]
}

variable "cors_allow_methods" {
  description = "CORS allowed HTTP methods"
  type        = list(string)
  default     = ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
}

variable "cors_allow_headers" {
  description = "CORS allowed headers"
  type        = list(string)
  default     = ["Content-Type", "Authorization", "X-Amz-Date", "X-Api-Key", "X-Amz-Security-Token"]
}

variable "cors_max_age" {
  description = "CORS preflight cache duration in seconds"
  type        = number
  default     = 86400
}

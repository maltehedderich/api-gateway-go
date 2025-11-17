# ========================================
# API Gateway Outputs
# ========================================

output "api_endpoint" {
  description = "API Gateway endpoint URL"
  value       = module.api_gateway.api_endpoint
}

output "api_id" {
  description = "API Gateway ID"
  value       = module.api_gateway.api_id
}

output "api_stage_name" {
  description = "API Gateway stage name"
  value       = module.api_gateway.stage_name
}

output "api_execution_arn" {
  description = "API Gateway execution ARN for Lambda permissions"
  value       = module.api_gateway.execution_arn
}

# ========================================
# Lambda Function Outputs
# ========================================

output "lambda_function_names" {
  description = "Names of Lambda functions"
  value       = { for k, v in module.lambda_functions : k => v.function_name }
}

output "lambda_function_arns" {
  description = "ARNs of Lambda functions"
  value       = { for k, v in module.lambda_functions : k => v.function_arn }
}

output "lambda_log_group_names" {
  description = "CloudWatch Log Group names for Lambda functions"
  value       = { for k, v in module.lambda_functions : k => v.log_group_name }
}

# ========================================
# DynamoDB Outputs
# ========================================

output "dynamodb_table_names" {
  description = "Names of DynamoDB tables"
  value       = var.enable_dynamodb ? { for k, v in module.dynamodb_tables : k => v.table_name } : {}
}

output "dynamodb_table_arns" {
  description = "ARNs of DynamoDB tables"
  value       = var.enable_dynamodb ? { for k, v in module.dynamodb_tables : k => v.table_arn } : {}
}

# ========================================
# Custom Domain Outputs
# ========================================

output "custom_domain_name" {
  description = "Custom domain name (if enabled)"
  value       = var.enable_custom_domain ? var.domain_name : null
}

output "custom_domain_api_endpoint" {
  description = "Custom domain API endpoint (if enabled)"
  value       = var.enable_custom_domain ? "https://${var.domain_name}" : null
}

# ========================================
# Environment Outputs
# ========================================

output "environment" {
  description = "Environment name"
  value       = var.environment
}

output "aws_region" {
  description = "AWS region"
  value       = var.aws_region
}

# ========================================
# Quick Start Information
# ========================================

output "quick_start_info" {
  description = "Quick start information for using the API"
  value = <<-EOT

    ╔════════════════════════════════════════════════════════════════╗
    ║           API Gateway Deployment - Quick Start                 ║
    ╚════════════════════════════════════════════════════════════════╝

    Environment: ${var.environment}
    Region:      ${var.aws_region}

    API Endpoint:
    ${module.api_gateway.api_endpoint}
    ${var.enable_custom_domain ? "\nCustom Domain:\n  https://${var.domain_name}" : ""}

    Example API Requests:

    1. Health Check (No Auth):
       curl ${module.api_gateway.api_endpoint}/_health

    2. Get Users (JWT Required):
       curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
            ${module.api_gateway.api_endpoint}/api/v1/users

    3. Create Order (JWT Required):
       curl -X POST \
            -H "Authorization: Bearer YOUR_JWT_TOKEN" \
            -H "Content-Type: application/json" \
            -d '{"product":"item1","quantity":2}' \
            ${module.api_gateway.api_endpoint}/api/v1/orders

    Lambda Functions Deployed:
    ${join("\n    ", [for k, v in module.lambda_functions : "- ${v.function_name}"])}

    DynamoDB Tables Created:
    ${var.enable_dynamodb ? join("\n    ", [for k, v in module.dynamodb_tables : "- ${v.table_name}"]) : "    (DynamoDB disabled)"}

    Next Steps:
    1. Upload Lambda function code to the functions
    2. Configure JWT issuer and audience
    3. Test API endpoints
    4. Monitor logs in CloudWatch

    CloudWatch Logs:
    - API Gateway: ${module.api_gateway.log_group_name}
    - Lambda Functions: /aws/lambda/${local.name_prefix}-*

  EOT
}

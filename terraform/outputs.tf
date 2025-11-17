# Terraform Outputs
# These values are displayed after successful deployment

output "api_endpoint" {
  description = "API Gateway endpoint URL"
  value       = module.api_gateway.api_endpoint
}

output "api_id" {
  description = "API Gateway ID"
  value       = module.api_gateway.api_id
}

output "custom_domain_url" {
  description = "Custom domain URL (if enabled)"
  value       = var.enable_custom_domain ? "https://${var.domain_name}" : "Not configured"
}

output "lambda_function_name" {
  description = "Lambda function name"
  value       = module.lambda.function_name
}

output "lambda_function_arn" {
  description = "Lambda function ARN"
  value       = module.lambda.function_arn
}

output "lambda_log_group" {
  description = "CloudWatch Log Group for Lambda"
  value       = module.lambda.log_group_name
}

output "dynamodb_table_name" {
  description = "DynamoDB table name for rate limiting"
  value       = module.dynamodb.table_name
}

output "dynamodb_table_arn" {
  description = "DynamoDB table ARN"
  value       = module.dynamodb.table_arn
}

output "deployment_summary" {
  description = "Summary of deployed resources"
  value = {
    region              = var.aws_region
    environment         = var.environment
    api_endpoint        = module.api_gateway.api_endpoint
    lambda_function     = module.lambda.function_name
    dynamodb_table      = module.dynamodb.table_name
    custom_domain       = var.enable_custom_domain ? var.domain_name : "disabled"
    log_level           = var.log_level
    authorization       = var.enable_authorization ? "enabled" : "disabled"
    rate_limiting       = var.enable_rate_limiting ? "enabled" : "disabled"
  }
}

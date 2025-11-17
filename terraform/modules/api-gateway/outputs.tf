output "api_id" {
  description = "API Gateway ID"
  value       = aws_apigatewayv2_api.this.id
}

output "api_endpoint" {
  description = "API Gateway endpoint URL"
  value       = aws_apigatewayv2_api.this.api_endpoint
}

output "stage_name" {
  description = "API Gateway stage name"
  value       = aws_apigatewayv2_stage.default.name
}

output "execution_arn" {
  description = "API Gateway execution ARN"
  value       = aws_apigatewayv2_api.this.execution_arn
}

output "log_group_name" {
  description = "CloudWatch Log Group name"
  value       = var.enable_access_logs ? aws_cloudwatch_log_group.api_gateway[0].name : null
}

output "log_group_arn" {
  description = "CloudWatch Log Group ARN"
  value       = var.enable_access_logs ? aws_cloudwatch_log_group.api_gateway[0].arn : null
}

output "authorizer_id" {
  description = "JWT Authorizer ID (if enabled)"
  value       = var.enable_jwt_authorizer ? aws_apigatewayv2_authorizer.jwt[0].id : null
}

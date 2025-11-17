output "function_name" {
  description = "Lambda function name"
  value       = aws_lambda_function.this.function_name
}

output "function_arn" {
  description = "Lambda function ARN"
  value       = aws_lambda_function.this.arn
}

output "invoke_arn" {
  description = "Lambda function invoke ARN"
  value       = aws_lambda_function.this.invoke_arn
}

output "qualified_arn" {
  description = "Lambda function qualified ARN"
  value       = aws_lambda_function.this.qualified_arn
}

output "version" {
  description = "Lambda function version"
  value       = aws_lambda_function.this.version
}

output "role_arn" {
  description = "IAM role ARN"
  value       = aws_iam_role.lambda.arn
}

output "role_name" {
  description = "IAM role name"
  value       = aws_iam_role.lambda.name
}

output "log_group_name" {
  description = "CloudWatch Log Group name"
  value       = aws_cloudwatch_log_group.lambda.name
}

output "log_group_arn" {
  description = "CloudWatch Log Group ARN"
  value       = aws_cloudwatch_log_group.lambda.arn
}

output "function_url" {
  description = "Lambda function URL (if enabled)"
  value       = var.enable_function_url ? aws_lambda_function_url.this[0].function_url : null
}

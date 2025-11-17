output "function_name" {
  description = "Name of the Lambda function"
  value       = aws_lambda_function.gateway.function_name
}

output "function_arn" {
  description = "ARN of the Lambda function"
  value       = aws_lambda_function.gateway.arn
}

output "invoke_arn" {
  description = "Invoke ARN of the Lambda function"
  value       = aws_lambda_function.gateway.invoke_arn
}

output "function_version" {
  description = "Latest version of the Lambda function"
  value       = aws_lambda_function.gateway.version
}

output "log_group_name" {
  description = "Name of the CloudWatch Log Group"
  value       = aws_cloudwatch_log_group.lambda.name
}

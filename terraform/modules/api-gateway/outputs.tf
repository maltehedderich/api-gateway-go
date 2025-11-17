output "api_id" {
  description = "ID of the API Gateway"
  value       = aws_apigatewayv2_api.gateway.id
}

output "api_endpoint" {
  description = "Endpoint URL of the API Gateway"
  value       = aws_apigatewayv2_api.gateway.api_endpoint
}

output "api_arn" {
  description = "ARN of the API Gateway"
  value       = aws_apigatewayv2_api.gateway.arn
}

output "stage_name" {
  description = "Name of the API Gateway stage"
  value       = aws_apigatewayv2_stage.default.name
}

output "stage_arn" {
  description = "ARN of the API Gateway stage"
  value       = aws_apigatewayv2_stage.default.arn
}

output "invoke_url" {
  description = "Full invoke URL for the API Gateway"
  value       = "${aws_apigatewayv2_api.gateway.api_endpoint}/${aws_apigatewayv2_stage.default.name}"
}

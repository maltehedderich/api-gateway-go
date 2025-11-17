output "domain_name" {
  description = "Custom domain name"
  value       = aws_apigatewayv2_domain_name.this.domain_name
}

output "domain_name_configuration" {
  description = "Domain name configuration"
  value       = aws_apigatewayv2_domain_name.this.domain_name_configuration
}

output "api_gateway_target_domain_name" {
  description = "API Gateway target domain name for DNS"
  value       = aws_apigatewayv2_domain_name.this.domain_name_configuration[0].target_domain_name
}

output "api_gateway_hosted_zone_id" {
  description = "API Gateway hosted zone ID for DNS"
  value       = aws_apigatewayv2_domain_name.this.domain_name_configuration[0].hosted_zone_id
}

output "api_endpoint_url" {
  description = "Custom domain API endpoint URL"
  value       = "https://${aws_apigatewayv2_domain_name.this.domain_name}"
}

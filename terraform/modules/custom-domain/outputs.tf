output "domain_name" {
  description = "Custom domain name"
  value       = aws_apigatewayv2_domain_name.api.domain_name
}

output "certificate_arn" {
  description = "ARN of the ACM certificate"
  value       = aws_acm_certificate.api.arn
}

output "target_domain_name" {
  description = "Target domain name for the API Gateway"
  value       = aws_apigatewayv2_domain_name.api.domain_name_configuration[0].target_domain_name
}

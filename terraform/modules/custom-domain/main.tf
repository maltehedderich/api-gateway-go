# ========================================
# API Gateway Custom Domain
# ========================================

resource "aws_apigatewayv2_domain_name" "this" {
  domain_name = var.domain_name

  domain_name_configuration {
    certificate_arn = var.certificate_arn
    endpoint_type   = "REGIONAL"
    security_policy = "TLS_1_2"
  }

  tags = var.tags
}

# ========================================
# API Mapping
# ========================================

resource "aws_apigatewayv2_api_mapping" "this" {
  api_id      = var.api_gateway_id
  domain_name = aws_apigatewayv2_domain_name.this.id
  stage       = var.api_gateway_stage_name
}

# ========================================
# Route53 DNS Record
# ========================================

resource "aws_route53_record" "api" {
  count = var.route53_zone_id != "" ? 1 : 0

  zone_id = var.route53_zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = aws_apigatewayv2_domain_name.this.domain_name_configuration[0].target_domain_name
    zone_id                = aws_apigatewayv2_domain_name.this.domain_name_configuration[0].hosted_zone_id
    evaluate_target_health = false
  }
}
